package types_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

const validHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

func validBatch() types.Batch {
	farmer := testAddr("farmer")
	return types.Batch{
		Id:             "ASH-001",
		HerbType:       "ashwagandha",
		Farmer:         farmer,
		Owner:          farmer,
		OriginLatMicro: 26_912_400,
		OriginLngMicro: 75_787_300,
		HarvestTime:    1_700_000_000,
		QuantityGrams:  25_000,
		QualityGrade:   "A",
		Certifications: []string{"organic"},
		Status:         types.StatusHarvested,
		Location:       "Jaipur",
		CreatedHeight:  1,
		UpdatedHeight:  1,
	}
}

func TestBatchValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.Batch)
		err    error
	}{
		{"valid", func(*types.Batch) {}, nil},
		{"valid boundaries", func(b *types.Batch) {
			b.Id, b.OriginLatMicro, b.OriginLngMicro = "A-1", -types.MaxLatMicro, types.MaxLngMicro
		}, nil},
		{"id lowercase", func(b *types.Batch) { b.Id = "ash-001" }, types.ErrInvalidBatch},
		{"id too short", func(b *types.Batch) { b.Id = "AB" }, types.ErrInvalidBatch},
		{"id too long", func(b *types.Batch) { b.Id = strings.Repeat("A", 65) }, types.ErrInvalidBatch},
		{"id bad char", func(b *types.Batch) { b.Id = "ASH_001" }, types.ErrInvalidBatch},
		{"herb type empty", func(b *types.Batch) { b.HerbType = "" }, types.ErrInvalidBatch},
		{"herb type too long", func(b *types.Batch) { b.HerbType = strings.Repeat("h", 65) }, types.ErrInvalidBatch},
		{"lat too high", func(b *types.Batch) { b.OriginLatMicro = types.MaxLatMicro + 1 }, types.ErrInvalidBatch},
		{"lat too low", func(b *types.Batch) { b.OriginLatMicro = -types.MaxLatMicro - 1 }, types.ErrInvalidBatch},
		{"lng too high", func(b *types.Batch) { b.OriginLngMicro = types.MaxLngMicro + 1 }, types.ErrInvalidBatch},
		{"lng too low", func(b *types.Batch) { b.OriginLngMicro = -types.MaxLngMicro - 1 }, types.ErrInvalidBatch},
		{"harvest time zero", func(b *types.Batch) { b.HarvestTime = 0 }, types.ErrInvalidBatch},
		{"quantity zero", func(b *types.Batch) { b.QuantityGrams = 0 }, types.ErrInvalidBatch},
		{"grade invalid", func(b *types.Batch) { b.QualityGrade = "D" }, types.ErrInvalidBatch},
		{"grade lowercase", func(b *types.Batch) { b.QualityGrade = "a" }, types.ErrInvalidBatch},
		{"too many certifications", func(b *types.Batch) { b.Certifications = make11Certs() }, types.ErrInvalidBatch},
		{"empty certification", func(b *types.Batch) { b.Certifications = []string{""} }, types.ErrInvalidBatch},
		{"certification too long", func(b *types.Batch) { b.Certifications = []string{strings.Repeat("c", 65)} }, types.ErrInvalidBatch},
		{"location too long", func(b *types.Batch) { b.Location = strings.Repeat("l", 129) }, types.ErrInvalidBatch},
		{"bad farmer", func(b *types.Batch) { b.Farmer = "bad" }, types.ErrInvalidAddress},
		{"bad owner", func(b *types.Batch) { b.Owner = "" }, types.ErrInvalidAddress},
		{"status unspecified", func(b *types.Batch) { b.Status = types.StatusUnspecified }, types.ErrInvalidBatch},
		{"hash before testing", func(b *types.Batch) { b.LabReportHash = validHash }, types.ErrInvalidBatch},
		{"tested without hash", func(b *types.Batch) { b.Status = types.StatusTested }, types.ErrInvalidReportHash},
		{"tested with hash", func(b *types.Batch) { b.Status, b.LabReportHash = types.StatusTested, validHash }, nil},
		{"negative height", func(b *types.Batch) { b.CreatedHeight = -1 }, types.ErrInvalidBatch},
		{"updated before created", func(b *types.Batch) { b.UpdatedHeight = 0 }, types.ErrInvalidBatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := validBatch()
			tc.mutate(&b)
			err := b.Validate(types.DefaultMaxCertifications)
			if tc.err == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func make11Certs() []string {
	certs := make([]string, 11)
	for i := range certs {
		certs[i] = "cert"
	}
	return certs
}

func TestValidateReportHash(t *testing.T) {
	require.NoError(t, types.ValidateReportHash(validHash))
	for _, h := range []string{"", validHash[:63], validHash + "0", strings.ToUpper(validHash), "z" + validHash[1:]} {
		require.ErrorIs(t, types.ValidateReportHash(h), types.ErrInvalidReportHash, h)
	}
}

func TestHistoryEntryValidate(t *testing.T) {
	valid := func() types.HistoryEntry {
		return types.HistoryEntry{
			BatchId: "ASH-001", Actor: testAddr("farmer"), Action: types.ActionCreated,
			FromStatus: types.StatusUnspecified, ToStatus: types.StatusHarvested, Height: 1, Time: 1,
		}
	}
	tests := []struct {
		name   string
		mutate func(*types.HistoryEntry)
		ok     bool
	}{
		{"valid", func(*types.HistoryEntry) {}, true},
		{"bad id", func(e *types.HistoryEntry) { e.BatchId = "x" }, false},
		{"bad actor", func(e *types.HistoryEntry) { e.Actor = "x" }, false},
		{"unspecified action", func(e *types.HistoryEntry) { e.Action = types.ActionUnspecified }, false},
		{"unknown action", func(e *types.HistoryEntry) { e.Action = 9 }, false},
		{"unspecified to", func(e *types.HistoryEntry) { e.ToStatus = types.StatusUnspecified }, false},
		{"unknown from", func(e *types.HistoryEntry) { e.FromStatus = 42 }, false},
		{"long location", func(e *types.HistoryEntry) { e.Location = strings.Repeat("l", 129) }, false},
		{"long note", func(e *types.HistoryEntry) { e.Note = strings.Repeat("n", types.MaxRemarkLen+1) }, false},
		{"negative time", func(e *types.HistoryEntry) { e.Time = -1 }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := valid()
			tc.mutate(&e)
			if tc.ok {
				require.NoError(t, e.Validate())
			} else {
				require.Error(t, e.Validate())
			}
		})
	}
}
