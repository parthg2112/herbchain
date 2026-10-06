package types

import (
	"regexp"

	errorsmod "cosmossdk.io/errors"
)

// Field limits from brief §3. String lengths are measured in bytes.
const (
	MaxHerbTypeLen      = 64
	MaxCertificationLen = 64
	MaxLocationLen      = 128
	MaxRemarkLen        = 256
	MaxLatMicro         = 90_000_000
	MaxLngMicro         = 180_000_000
	ReportHashLen       = 64
)

var (
	batchIDRegex    = regexp.MustCompile(`^[A-Z0-9-]{3,64}$`)
	reportHashRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ValidateBatchID checks a batch id against ^[A-Z0-9-]{3,64}$.
func ValidateBatchID(id string) error {
	if !batchIDRegex.MatchString(id) {
		return errorsmod.Wrapf(ErrInvalidBatch, "id %q must match %s", id, batchIDRegex)
	}
	return nil
}

// ValidateReportHash requires a lowercase hex sha256 digest.
func ValidateReportHash(h string) error {
	if !reportHashRegex.MatchString(h) {
		return errorsmod.Wrapf(ErrInvalidReportHash, "%q must be %d lowercase hex chars", h, ReportHashLen)
	}
	return nil
}

// ValidateLocation bounds the free-text location.
func ValidateLocation(s string) error {
	if len(s) > MaxLocationLen {
		return errorsmod.Wrapf(ErrInvalidBatch, "location longer than %d bytes", MaxLocationLen)
	}
	return nil
}

// ValidateRemark bounds the free-text remark stored as HistoryEntry.note.
func ValidateRemark(s string) error {
	if len(s) > MaxRemarkLen {
		return errorsmod.Wrapf(ErrInvalidBatch, "remark longer than %d bytes", MaxRemarkLen)
	}
	return nil
}

// ValidateCertifications checks count against maxCount and each entry's length.
func ValidateCertifications(certs []string, maxCount uint32) error {
	if uint64(len(certs)) > uint64(maxCount) {
		return errorsmod.Wrapf(ErrInvalidBatch, "%d certifications exceeds max %d", len(certs), maxCount)
	}
	for i, c := range certs {
		if c == "" || len(c) > MaxCertificationLen {
			return errorsmod.Wrapf(ErrInvalidBatch, "certification %d must be 1-%d bytes", i, MaxCertificationLen)
		}
	}
	return nil
}

// ValidateFields checks the user-supplied batch fields; addresses, status and heights are not checked.
func (b Batch) ValidateFields(maxCertifications uint32) error {
	if err := ValidateBatchID(b.Id); err != nil {
		return err
	}
	if b.HerbType == "" || len(b.HerbType) > MaxHerbTypeLen {
		return errorsmod.Wrapf(ErrInvalidBatch, "herb_type must be 1-%d bytes", MaxHerbTypeLen)
	}
	if b.OriginLatMicro < -MaxLatMicro || b.OriginLatMicro > MaxLatMicro {
		return errorsmod.Wrapf(ErrInvalidBatch, "origin_lat_micro %d out of range ±%d", b.OriginLatMicro, MaxLatMicro)
	}
	if b.OriginLngMicro < -MaxLngMicro || b.OriginLngMicro > MaxLngMicro {
		return errorsmod.Wrapf(ErrInvalidBatch, "origin_lng_micro %d out of range ±%d", b.OriginLngMicro, MaxLngMicro)
	}
	if b.HarvestTime <= 0 {
		return errorsmod.Wrapf(ErrInvalidBatch, "harvest_time %d must be positive unix seconds", b.HarvestTime)
	}
	if b.QuantityGrams == 0 {
		return errorsmod.Wrap(ErrInvalidBatch, "quantity_grams must be > 0")
	}
	switch b.QualityGrade {
	case "A", "B", "C":
	default:
		return errorsmod.Wrapf(ErrInvalidBatch, "quality_grade %q must be A, B or C", b.QualityGrade)
	}
	if err := ValidateCertifications(b.Certifications, maxCertifications); err != nil {
		return err
	}
	return ValidateLocation(b.Location)
}

// Validate checks a fully stored batch, e.g. from genesis.
func (b Batch) Validate(maxCertifications uint32) error {
	if err := b.ValidateFields(maxCertifications); err != nil {
		return err
	}
	if err := ValidateCanonicalAddress(b.Farmer); err != nil {
		return errorsmod.Wrapf(err, "batch %s farmer", b.Id)
	}
	if err := ValidateCanonicalAddress(b.Owner); err != nil {
		return errorsmod.Wrapf(err, "batch %s owner", b.Id)
	}
	if !IsValidStatus(b.Status) {
		return errorsmod.Wrapf(ErrInvalidBatch, "batch %s has invalid status %s", b.Id, b.Status)
	}
	if err := b.validateReportHash(); err != nil {
		return err
	}
	if b.CreatedHeight < 0 || b.UpdatedHeight < b.CreatedHeight {
		return errorsmod.Wrapf(ErrInvalidBatch, "batch %s heights created=%d updated=%d", b.Id, b.CreatedHeight, b.UpdatedHeight)
	}
	return nil
}

// validateReportHash enforces that a hash exists exactly when a lab result has been recorded.
func (b Batch) validateReportHash() error {
	tested := b.Status != StatusHarvested && b.Status != StatusProcessed
	if !tested {
		if b.LabReportHash != "" {
			return errorsmod.Wrapf(ErrInvalidBatch, "batch %s has a report hash before testing", b.Id)
		}
		return nil
	}
	return ValidateReportHash(b.LabReportHash)
}

// Validate checks a history entry's self-contained fields.
func (e HistoryEntry) Validate() error {
	if err := ValidateBatchID(e.BatchId); err != nil {
		return err
	}
	if err := ValidateCanonicalAddress(e.Actor); err != nil {
		return errorsmod.Wrapf(err, "history %s/%d actor", e.BatchId, e.Seq)
	}
	if e.Action < ActionCreated || e.Action > ActionLabResult {
		return errorsmod.Wrapf(ErrInvalidBatch, "history %s/%d has invalid action %s", e.BatchId, e.Seq, e.Action)
	}
	if !IsValidStatus(e.ToStatus) || (e.FromStatus != StatusUnspecified && !IsValidStatus(e.FromStatus)) {
		return errorsmod.Wrapf(ErrInvalidBatch, "history %s/%d has invalid statuses", e.BatchId, e.Seq)
	}
	if err := ValidateLocation(e.Location); err != nil {
		return err
	}
	if err := ValidateRemark(e.Note); err != nil {
		return err
	}
	if e.Height < 0 || e.Time < 0 {
		return errorsmod.Wrapf(ErrInvalidBatch, "history %s/%d has negative height or time", e.BatchId, e.Seq)
	}
	return nil
}
