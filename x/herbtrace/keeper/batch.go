package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

// GetBatch returns the batch with the given id, or an error wrapping types.ErrBatchNotFound.
func (k Keeper) GetBatch(ctx context.Context, id string) (types.Batch, error) {
	b, err := k.Batches.Get(ctx, id)
	if errors.Is(err, collections.ErrNotFound) {
		return types.Batch{}, errorsmod.Wrapf(types.ErrBatchNotFound, "id %s", id)
	}
	if err != nil {
		return types.Batch{}, errorsmod.Wrapf(err, "get batch %s", id)
	}
	return b, nil
}

// setBatch is the single write path for Batches and keeps OwnerIndex/StatusIndex in sync.
// old is the previously stored value, or nil on creation.
func (k Keeper) setBatch(ctx context.Context, old *types.Batch, b types.Batch) error {
	owner, err := k.addressCodec.StringToBytes(b.Owner)
	if err != nil {
		return errorsmod.Wrapf(types.ErrInvalidAddress, "batch %s owner %q: %s", b.Id, b.Owner, err)
	}
	ownerChanged := old == nil || old.Owner != b.Owner
	statusChanged := old == nil || old.Status != b.Status

	if old != nil && ownerChanged {
		oldOwner, err := k.addressCodec.StringToBytes(old.Owner)
		if err != nil {
			return errorsmod.Wrapf(types.ErrInvalidAddress, "batch %s old owner %q: %s", b.Id, old.Owner, err)
		}
		if err := k.OwnerIndex.Remove(ctx, collections.Join(sdk.AccAddress(oldOwner), b.Id)); err != nil {
			return errorsmod.Wrapf(err, "remove owner index for %s", b.Id)
		}
	}
	if old != nil && statusChanged {
		if err := k.StatusIndex.Remove(ctx, collections.Join(int32(old.Status), b.Id)); err != nil {
			return errorsmod.Wrapf(err, "remove status index for %s", b.Id)
		}
	}

	if err := k.Batches.Set(ctx, b.Id, b); err != nil {
		return errorsmod.Wrapf(err, "set batch %s", b.Id)
	}
	if ownerChanged {
		if err := k.OwnerIndex.Set(ctx, collections.Join(sdk.AccAddress(owner), b.Id)); err != nil {
			return errorsmod.Wrapf(err, "set owner index for %s", b.Id)
		}
	}
	if statusChanged {
		if err := k.StatusIndex.Set(ctx, collections.Join(int32(b.Status), b.Id)); err != nil {
			return errorsmod.Wrapf(err, "set status index for %s", b.Id)
		}
	}
	return nil
}

// appendHistory assigns the batch's next seq to e and stores it.
func (k Keeper) appendHistory(ctx context.Context, e types.HistoryEntry) error {
	seq, err := k.HistorySeq.Get(ctx, e.BatchId)
	switch {
	case errors.Is(err, collections.ErrNotFound):
		seq = 0
	case err != nil:
		return errorsmod.Wrapf(err, "get history seq for %s", e.BatchId)
	}
	e.Seq = seq
	if err := k.History.Set(ctx, collections.Join(e.BatchId, seq), e); err != nil {
		return errorsmod.Wrapf(err, "set history %s/%d", e.BatchId, seq)
	}
	return k.HistorySeq.Set(ctx, e.BatchId, seq+1)
}

// newHistoryEntry stamps an entry with the current block height and time.
func newHistoryEntry(ctx context.Context, b types.Batch, actor string, action types.HistoryAction,
	from types.BatchStatus, note string,
) types.HistoryEntry {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return types.HistoryEntry{
		BatchId:    b.Id,
		Actor:      actor,
		Action:     action,
		FromStatus: from,
		ToStatus:   b.Status,
		Location:   b.Location,
		Note:       note,
		Height:     sdkCtx.BlockHeight(),
		Time:       sdkCtx.BlockTime().Unix(),
	}
}

// canonicalAddress decodes s with the keeper's codec and re-encodes it, so stored addresses
// compare equal as strings iff they are the same account.
func (k Keeper) canonicalAddress(s, field string) (string, error) {
	bz, err := k.addressCodec.StringToBytes(s)
	if err != nil {
		return "", errorsmod.Wrapf(types.ErrInvalidAddress, "%s %q: %s", field, s, err)
	}
	out, err := k.addressCodec.BytesToString(bz)
	if err != nil {
		return "", errorsmod.Wrapf(types.ErrInvalidAddress, "%s %q: %s", field, s, err)
	}
	return out, nil
}

// isCertifier relies on params certifiers being canonical (enforced by Params.Validate).
func isCertifier(params types.Params, addr string) bool {
	for _, c := range params.Certifiers {
		if c == addr {
			return true
		}
	}
	return false
}
