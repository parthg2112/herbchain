package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func (k msgServer) RecordLabResult(ctx context.Context, msg *types.MsgRecordLabResult) (*types.MsgRecordLabResultResponse, error) {
	signer, err := k.canonicalAddress(msg.Certifier, "certifier")
	if err != nil {
		return nil, err
	}
	if err := types.ValidateReportHash(msg.ReportHash); err != nil {
		return nil, err
	}
	if err := types.ValidateRemark(msg.Remark); err != nil {
		return nil, err
	}

	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "get params")
	}
	b, err := k.GetBatch(ctx, msg.BatchId)
	if err != nil {
		return nil, err
	}
	if !isCertifier(params, signer) {
		return nil, errorsmod.Wrapf(types.ErrUnauthorized, "%s is not a certifier", signer)
	}

	target := types.StatusRejected
	if msg.Passed {
		target = types.StatusTested
	}
	if err := types.ValidateTransition(b.Status, target); err != nil {
		return nil, errorsmod.Wrapf(err, "batch %s", b.Id)
	}

	old := b
	b.Status = target
	b.LabReportHash = msg.ReportHash
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	b.UpdatedHeight = sdkCtx.BlockHeight()
	if err := k.setBatch(ctx, &old, b); err != nil {
		return nil, err
	}
	entry := newHistoryEntry(ctx, b, signer, types.ActionLabResult, old.Status, msg.Remark)
	if err := k.appendHistory(ctx, entry); err != nil {
		return nil, err
	}

	if err := sdkCtx.EventManager().EmitTypedEvent(&types.EventLabResultRecorded{
		BatchId:    b.Id,
		Certifier:  signer,
		Passed:     msg.Passed,
		ReportHash: msg.ReportHash,
		ToStatus:   target,
	}); err != nil {
		return nil, errorsmod.Wrap(err, "emit EventLabResultRecorded")
	}
	return &types.MsgRecordLabResultResponse{Status: target}, nil
}
