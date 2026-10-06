package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func validGenesis() *types.GenesisState {
	b1 := validBatch()
	b2 := validBatch()
	b2.Id = "TUL-002"
	entry := func(id string, seq uint64) types.HistoryEntry {
		return types.HistoryEntry{
			BatchId: id, Seq: seq, Actor: b1.Farmer, Action: types.ActionCreated,
			ToStatus: types.StatusHarvested, Height: 1, Time: 1,
		}
	}
	return &types.GenesisState{
		Params:  types.NewParams([]string{testAddr("carol")}, 10),
		Batches: []types.Batch{b1, b2},
		// Interleaved order is accepted; only uniqueness and contiguity matter.
		History: []types.HistoryEntry{entry(b2.Id, 0), entry(b1.Id, 1), entry(b1.Id, 0)},
		HistorySeqs: []types.HistorySeq{
			{BatchId: b1.Id, NextSeq: 2},
			{BatchId: b2.Id, NextSeq: 1},
		},
	}
}

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc   string
		mutate func(*types.GenesisState)
		valid  bool
	}{
		{"valid", func(*types.GenesisState) {}, true},
		{"invalid params", func(gs *types.GenesisState) { gs.Params.MaxCertifications = 0 }, false},
		{"duplicate batch id", func(gs *types.GenesisState) { gs.Batches[1].Id = gs.Batches[0].Id }, false},
		{"invalid batch", func(gs *types.GenesisState) { gs.Batches[0].QuantityGrams = 0 }, false},
		{"batch exceeds max certifications", func(gs *types.GenesisState) {
			gs.Params.MaxCertifications = 1
			gs.Batches[0].Certifications = []string{"a", "b"}
		}, false},
		{"history for unknown batch", func(gs *types.GenesisState) { gs.History[0].BatchId = "NOPE-1" }, false},
		{"invalid history entry", func(gs *types.GenesisState) { gs.History[0].Action = types.ActionUnspecified }, false},
		{"duplicate history entry", func(gs *types.GenesisState) { gs.History[2].Seq = 1 }, false},
		{"history seq gap", func(gs *types.GenesisState) { gs.HistorySeqs[0].NextSeq = 3 }, false},
		{"history beyond next seq", func(gs *types.GenesisState) { gs.History[1].Seq = 5 }, false},
		{"missing history entry", func(gs *types.GenesisState) { gs.History = gs.History[:2] }, false},
		{"seq for unknown batch", func(gs *types.GenesisState) { gs.HistorySeqs[1].BatchId = "NOPE-1" }, false},
		{"duplicate seq entry", func(gs *types.GenesisState) { gs.HistorySeqs[1].BatchId = gs.HistorySeqs[0].BatchId }, false},
		{"batch without seq", func(gs *types.GenesisState) { gs.HistorySeqs = gs.HistorySeqs[:1] }, false},
		{"zero next seq", func(gs *types.GenesisState) { gs.HistorySeqs[1].NextSeq = 0 }, false},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			gs := validGenesis()
			tc.mutate(gs)
			err := gs.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
		})
	}
}

func TestDefaultGenesisValid(t *testing.T) {
	require.NoError(t, types.DefaultGenesis().Validate())
	require.Error(t, (&types.GenesisState{}).Validate(), "zero params are invalid")
}
