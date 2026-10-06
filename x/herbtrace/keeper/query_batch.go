package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func (q queryServer) Batch(ctx context.Context, req *types.QueryBatchRequest) (*types.QueryBatchResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "batch id is required")
	}
	b, err := q.k.GetBatch(ctx, req.Id)
	if err != nil {
		return nil, batchLookupError(err)
	}
	return &types.QueryBatchResponse{Batch: b}, nil
}

func (q queryServer) Batches(ctx context.Context, req *types.QueryBatchesRequest) (*types.QueryBatchesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if err := validatePageRequest(req.Pagination); err != nil {
		return nil, err
	}
	batches, page, err := query.CollectionPaginate(ctx, q.k.Batches, req.Pagination,
		func(_ string, b types.Batch) (types.Batch, error) { return b, nil })
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryBatchesResponse{Batches: batches, Pagination: page}, nil
}

func (q queryServer) BatchHistory(ctx context.Context, req *types.QueryBatchHistoryRequest) (*types.QueryBatchHistoryResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "batch id is required")
	}
	if err := validatePageRequest(req.Pagination); err != nil {
		return nil, err
	}
	if _, err := q.k.GetBatch(ctx, req.Id); err != nil {
		return nil, batchLookupError(err)
	}
	entries, page, err := query.CollectionPaginate(ctx, q.k.History, req.Pagination,
		func(_ collections.Pair[string, uint64], e types.HistoryEntry) (types.HistoryEntry, error) {
			return e, nil
		},
		query.WithCollectionPaginationPairPrefix[string, uint64](req.Id))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryBatchHistoryResponse{Entries: entries, Pagination: page}, nil
}

func (q queryServer) BatchesByOwner(ctx context.Context, req *types.QueryBatchesByOwnerRequest) (*types.QueryBatchesByOwnerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	owner, err := q.k.addressCodec.StringToBytes(req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid owner address %q: %s", req.Owner, err)
	}
	if err := validatePageRequest(req.Pagination); err != nil {
		return nil, err
	}
	batches, page, err := query.CollectionPaginate(ctx, q.k.OwnerIndex, req.Pagination,
		func(key collections.Pair[sdk.AccAddress, string], _ collections.NoValue) (types.Batch, error) {
			return q.k.Batches.Get(ctx, key.K2())
		},
		query.WithCollectionPaginationPairPrefix[sdk.AccAddress, string](owner))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryBatchesByOwnerResponse{Batches: batches, Pagination: page}, nil
}

func (q queryServer) BatchesByStatus(ctx context.Context, req *types.QueryBatchesByStatusRequest) (*types.QueryBatchesByStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if !types.IsValidStatus(req.Status) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid status %s", req.Status)
	}
	if err := validatePageRequest(req.Pagination); err != nil {
		return nil, err
	}
	batches, page, err := query.CollectionPaginate(ctx, q.k.StatusIndex, req.Pagination,
		func(key collections.Pair[int32, string], _ collections.NoValue) (types.Batch, error) {
			return q.k.Batches.Get(ctx, key.K2())
		},
		query.WithCollectionPaginationPairPrefix[int32, string](int32(req.Status)))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryBatchesByStatusResponse{Batches: batches, Pagination: page}, nil
}

func validatePageRequest(p *query.PageRequest) error {
	if p != nil && p.Offset > 0 && len(p.Key) > 0 {
		return status.Error(codes.InvalidArgument, "pagination: set either offset or key, not both")
	}
	return nil
}

func batchLookupError(err error) error {
	if errors.Is(err, types.ErrBatchNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
