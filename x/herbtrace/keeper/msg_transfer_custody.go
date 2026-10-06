package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func (k msgServer) TransferCustody(ctx context.Context, msg *types.MsgTransferCustody) (*types.MsgTransferCustodyResponse, error) {
	signer, err := k.canonicalAddress(msg.Owner, "owner")
	if err != nil {
		return nil, err
	}
	newOwner, err := k.canonicalAddress(msg.NewOwner, "new_owner")
	if err != nil {
		return nil, err
	}
	if newOwner == signer {
		return nil, errorsmod.Wrap(types.ErrInvalidAddress, "new_owner must differ from owner")
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

	old := b
	b.Owner = newOwner
	if msg.Location != "" {
		b.Location = msg.Location
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	b.UpdatedHeight = sdkCtx.BlockHeight()
	if err := k.setBatch(ctx, &old, b); err != nil {
		return nil, err
	}
	entry := newHistoryEntry(ctx, b, signer, types.ActionCustodyTransferred, old.Status, msg.Remark)
	if err := k.appendHistory(ctx, entry); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventCustodyTransferred{
		BatchId:   b.Id,
		FromOwner: signer,
		ToOwner:   newOwner,
	}); err != nil {
		return nil, errorsmod.Wrap(err, "emit EventCustodyTransferred")
	}
	return &types.MsgTransferCustodyResponse{}, nil
}
