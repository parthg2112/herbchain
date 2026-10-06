package keeper

// TEMPORARY: compile stubs so x/attest can build in parallel. Delete when real handlers land.

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

var errStub = status.Error(codes.Unimplemented, "not implemented")

// GetBatch is the read API consumed by x/attest (contract: returns wrapped types.ErrBatchNotFound when absent).
func (k Keeper) GetBatch(ctx context.Context, id string) (types.Batch, error) {
	return types.Batch{}, errStub
}

func (msgServer) CreateBatch(context.Context, *types.MsgCreateBatch) (*types.MsgCreateBatchResponse, error) {
	return nil, errStub
}

func (msgServer) UpdateStatus(context.Context, *types.MsgUpdateStatus) (*types.MsgUpdateStatusResponse, error) {
	return nil, errStub
}

func (msgServer) RecordLabResult(context.Context, *types.MsgRecordLabResult) (*types.MsgRecordLabResultResponse, error) {
	return nil, errStub
}

func (msgServer) TransferCustody(context.Context, *types.MsgTransferCustody) (*types.MsgTransferCustodyResponse, error) {
	return nil, errStub
}

func (queryServer) Batch(context.Context, *types.QueryBatchRequest) (*types.QueryBatchResponse, error) {
	return nil, errStub
}

func (queryServer) Batches(context.Context, *types.QueryBatchesRequest) (*types.QueryBatchesResponse, error) {
	return nil, errStub
}

func (queryServer) BatchHistory(context.Context, *types.QueryBatchHistoryRequest) (*types.QueryBatchHistoryResponse, error) {
	return nil, errStub
}

func (queryServer) BatchesByOwner(context.Context, *types.QueryBatchesByOwnerRequest) (*types.QueryBatchesByOwnerResponse, error) {
	return nil, errStub
}

func (queryServer) BatchesByStatus(context.Context, *types.QueryBatchesByStatusRequest) (*types.QueryBatchesByStatusResponse, error) {
	return nil, errStub
}
