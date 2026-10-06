package types

import (
	"regexp"
	"slices"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

// Wire status vocabulary; decoupled from the herbtrace enum so counterparties need not share its proto.
const (
	StatusTested    = "TESTED"
	StatusPackaged  = "PACKAGED"
	StatusShipped   = "SHIPPED"
	StatusDelivered = "DELIVERED"
)

const (
	maxHerbTypeLen = 64
	maxFarmerLen   = 128
)

var (
	attestableStatuses = []string{StatusTested, StatusPackaged, StatusShipped, StatusDelivered}
	batchIDRegexp      = regexp.MustCompile(`^[A-Z0-9-]{3,64}$`)
	sha256HexRegexp    = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// IsAttestableStatus reports whether status is a valid wire status.
func IsAttestableStatus(status string) bool {
	return slices.Contains(attestableStatuses, status)
}

// ValidateBatchID checks the herbtrace batch id format.
func ValidateBatchID(id string) error {
	if !batchIDRegexp.MatchString(id) {
		return errorsmod.Wrapf(ErrInvalidPacket, "batch id %q must match %s", id, batchIDRegexp)
	}
	return nil
}

// GetBytes is a helper for serialising
func (p AttestationPacketData) GetBytes() ([]byte, error) {
	var modulePacket AttestPacketData

	modulePacket.Packet = &AttestPacketData_AttestationPacket{&p}

	return modulePacket.Marshal()
}

// ValidateBasic checks packet fields without any state access; run on both send and receive.
func (p AttestationPacketData) ValidateBasic() error {
	if err := ValidateBatchID(p.BatchId); err != nil {
		return err
	}
	if p.HerbType == "" || len(p.HerbType) > maxHerbTypeLen {
		return errorsmod.Wrapf(ErrInvalidPacket, "herb type must be 1-%d bytes", maxHerbTypeLen)
	}
	// Farmer is bech32 under the source chain's prefix, which the receiver cannot know.
	if len(p.Farmer) > maxFarmerLen {
		return errorsmod.Wrapf(ErrInvalidPacket, "farmer exceeds %d bytes", maxFarmerLen)
	}
	if _, _, err := bech32.DecodeAndConvert(p.Farmer); err != nil {
		return errorsmod.Wrapf(ErrInvalidPacket, "farmer %q is not bech32: %s", p.Farmer, err)
	}
	if !IsAttestableStatus(p.Status) {
		return errorsmod.Wrapf(ErrInvalidPacket, "status %q is not one of %v", p.Status, attestableStatuses)
	}
	if !sha256HexRegexp.MatchString(p.LabReportHash) {
		return errorsmod.Wrap(ErrInvalidPacket, "lab report hash must be 64 hex chars")
	}
	if p.SourceHeight <= 0 {
		return errorsmod.Wrapf(ErrInvalidPacket, "source height must be positive, got %d", p.SourceHeight)
	}
	return nil
}
