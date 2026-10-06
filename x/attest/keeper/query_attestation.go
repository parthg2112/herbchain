package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/parthg2112/herbchain/x/attest/types"
)

func (q queryServer) Attestation(ctx context.Context, req *types.QueryAttestationRequest) (*types.QueryAttestationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	att, err := q.k.Attestations.Get(ctx, collections.Join(req.ChannelId, req.BatchId))
	if err != nil {
		return nil, lookupErr(err, "attestation", req.ChannelId, req.BatchId)
	}
	return &types.QueryAttestationResponse{Attestation: att}, nil
}

func (q queryServer) Attestations(ctx context.Context, req *types.QueryAttestationsRequest) (*types.QueryAttestationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	atts, pageRes, err := query.CollectionPaginate(ctx, q.k.Attestations, req.Pagination,
		func(_ collections.Pair[string, string], v types.Attestation) (types.Attestation, error) {
			return v, nil
		})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &types.QueryAttestationsResponse{Attestations: atts, Pagination: pageRes}, nil
}

func (q queryServer) SentAttestation(ctx context.Context, req *types.QuerySentAttestationRequest) (*types.QuerySentAttestationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sent, err := q.k.SentAttestations.Get(ctx, collections.Join(req.ChannelId, req.BatchId))
	if err != nil {
		return nil, lookupErr(err, "sent attestation", req.ChannelId, req.BatchId)
	}
	return &types.QuerySentAttestationResponse{SentAttestation: sent}, nil
}

func (q queryServer) SentAttestations(ctx context.Context, req *types.QuerySentAttestationsRequest) (*types.QuerySentAttestationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sents, pageRes, err := query.CollectionPaginate(ctx, q.k.SentAttestations, req.Pagination,
		func(_ collections.Pair[string, string], v types.SentAttestation) (types.SentAttestation, error) {
			return v, nil
		})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &types.QuerySentAttestationsResponse{SentAttestations: sents, Pagination: pageRes}, nil
}

func lookupErr(err error, what, channelID, batchID string) error {
	if errors.Is(err, collections.ErrNotFound) {
		return status.Errorf(codes.NotFound, "%s %s/%s not found", what, channelID, batchID)
	}
	return status.Error(codes.Internal, err.Error())
}
