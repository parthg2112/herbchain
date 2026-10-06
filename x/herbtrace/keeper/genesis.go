package keeper

import (
	"context"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

// InitGenesis writes params, batches, history and seqs, and rebuilds the owner/status indexes.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	if err := k.Params.Set(ctx, gs.Params); err != nil {
		return errorsmod.Wrap(err, "set params")
	}
	for _, b := range gs.Batches {
		if err := k.setBatch(ctx, nil, b); err != nil {
			return err
		}
	}
	for _, e := range gs.History {
		if err := k.History.Set(ctx, collections.Join(e.BatchId, e.Seq), e); err != nil {
			return errorsmod.Wrapf(err, "set history %s/%d", e.BatchId, e.Seq)
		}
	}
	for _, s := range gs.HistorySeqs {
		if err := k.HistorySeq.Set(ctx, s.BatchId, s.NextSeq); err != nil {
			return errorsmod.Wrapf(err, "set history seq %s", s.BatchId)
		}
	}
	return nil
}

// ExportGenesis exports params, batches, history and seqs in store (key) order. Indexes are derived.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "get params")
	}
	gs := &types.GenesisState{Params: params}

	err = k.Batches.Walk(ctx, nil, func(_ string, b types.Batch) (bool, error) {
		gs.Batches = append(gs.Batches, b)
		return false, nil
	})
	if err != nil {
		return nil, errorsmod.Wrap(err, "export batches")
	}
	err = k.History.Walk(ctx, nil, func(_ collections.Pair[string, uint64], e types.HistoryEntry) (bool, error) {
		gs.History = append(gs.History, e)
		return false, nil
	})
	if err != nil {
		return nil, errorsmod.Wrap(err, "export history")
	}
	err = k.HistorySeq.Walk(ctx, nil, func(id string, next uint64) (bool, error) {
		gs.HistorySeqs = append(gs.HistorySeqs, types.HistorySeq{BatchId: id, NextSeq: next})
		return false, nil
	})
	if err != nil {
		return nil, errorsmod.Wrap(err, "export history seqs")
	}
	return gs, nil
}
