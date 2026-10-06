package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func (k msgServer) UpdateStatus(ctx context.Context, msg *types.MsgUpdateStatus) (*types.MsgUpdateStatusResponse, error) {
	signer, err := k.canonicalAddress(msg.Owner, "owner")
	if err != nil {
		return nil, err
	}
	if err := types.ValidateLocation(msg.Location); err != nil {
		return nil, err
	}
	if err := types.ValidateRemark(msg.Remark); err != nil {
		return nil, err
	}

	b, err := k.GetBatch(ctx, msg.BatchId)
	if err != nil {
		return nil, err
	}
	if b.Owner != signer {
		return nil, errorsmod.Wrapf(types.ErrUnauthorized, "%s is not the owner of batch %s", signer, b.Id)
	}
	if b.Status.IsTerminal() {
		return nil, errorsmod.Wrapf(types.ErrTerminalStatus, "batch %s is %s", b.Id, b.Status)
	}
	if !types.IsOwnerDrivenStatus(msg.Status) {
		return nil, errorsmod.Wrapf(types.ErrInvalidTransition,
			"%s cannot be set by the owner; TESTED/REJECTED require MsgRecordLabResult", msg.Status)
	}
	if err := types.ValidateTransition(b.Status, msg.Status); err != nil {
		return nil, errorsmod.Wrapf(err, "batch %s", b.Id)
	}

	old := b
	b.Status = msg.Status
	if msg.Location != "" {
		b.Location = msg.Location
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	b.UpdatedHeight = sdkCtx.BlockHeight()
	if err := k.setBatch(ctx, &old, b); err != nil {
		return nil, err
	}
	entry := newHistoryEntry(ctx, b, signer, types.ActionStatusUpdated, old.Status, msg.Remark)
	if err := k.appendHistory(ctx, entry); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventStatusUpdated{
		BatchId:    b.Id,
		Actor:      signer,
		FromStatus: old.Status,
		ToStatus:   b.Status,
		Location:   b.Location,
	}); err != nil {
		return nil, errorsmod.Wrap(err, "emit EventStatusUpdated")
	}
	return &types.MsgUpdateStatusResponse{}, nil
}
