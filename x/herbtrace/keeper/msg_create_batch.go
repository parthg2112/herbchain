package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func (k msgServer) CreateBatch(ctx context.Context, msg *types.MsgCreateBatch) (*types.MsgCreateBatchResponse, error) {
	farmer, err := k.canonicalAddress(msg.Farmer, "farmer")
	if err != nil {
		return nil, err
	}
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "get params")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	b := types.Batch{
		Id:             msg.Id,
		HerbType:       msg.HerbType,
		Farmer:         farmer,
		Owner:          farmer,
		OriginLatMicro: msg.OriginLatMicro,
		OriginLngMicro: msg.OriginLngMicro,
		HarvestTime:    msg.HarvestTime,
		QuantityGrams:  msg.QuantityGrams,
		QualityGrade:   msg.QualityGrade,
		Certifications: msg.Certifications,
		Status:         types.StatusHarvested,
		Location:       msg.Location,
		CreatedHeight:  sdkCtx.BlockHeight(),
		UpdatedHeight:  sdkCtx.BlockHeight(),
	}
	if err := b.ValidateFields(params.MaxCertifications); err != nil {
		return nil, err
	}
	if now := sdkCtx.BlockTime().Unix(); b.HarvestTime > now {
		return nil, errorsmod.Wrapf(types.ErrInvalidBatch, "harvest_time %d is after block time %d", b.HarvestTime, now)
	}

	exists, err := k.Batches.Has(ctx, b.Id)
	if err != nil {
		return nil, errorsmod.Wrapf(err, "check batch %s", b.Id)
	}
	if exists {
		return nil, errorsmod.Wrapf(types.ErrBatchExists, "id %s", b.Id)
	}

	if err := k.setBatch(ctx, nil, b); err != nil {
		return nil, err
	}
	entry := newHistoryEntry(ctx, b, farmer, types.ActionCreated, types.StatusUnspecified, "")
	if err := k.appendHistory(ctx, entry); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventBatchCreated{
		BatchId:       b.Id,
		Farmer:        farmer,
		HerbType:      b.HerbType,
		QuantityGrams: b.QuantityGrams,
	}); err != nil {
		return nil, errorsmod.Wrap(err, "emit EventBatchCreated")
	}
	return &types.MsgCreateBatchResponse{}, nil
}
