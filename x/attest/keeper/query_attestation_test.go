package keeper_test

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/parthg2112/herbchain/x/attest/keeper"
	"github.com/parthg2112/herbchain/x/attest/types"
	herbtracetypes "github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestQueryAttestations(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)
	for i := range 5 {
		_, err := f.keeper.OnRecvAttestationPacket(f.ctx, recvPacket(testChannel, uint64(i+1)), f.packetData(fmt.Sprintf("BATCH-%d", i)))
		require.NoError(t, err)
	}

	res, err := qs.Attestation(f.ctx, &types.QueryAttestationRequest{ChannelId: testChannel, BatchId: "BATCH-2"})
	require.NoError(t, err)
	require.Equal(t, uint64(3), res.Attestation.Sequence)

	_, err = qs.Attestation(f.ctx, &types.QueryAttestationRequest{ChannelId: "channel-7", BatchId: "BATCH-2"})
	require.Equal(t, codes.NotFound, status.Code(err))

	page1, err := qs.Attestations(f.ctx, &types.QueryAttestationsRequest{Pagination: &query.PageRequest{Limit: 3, CountTotal: true}})
	require.NoError(t, err)
	require.Len(t, page1.Attestations, 3)
	require.Equal(t, uint64(5), page1.Pagination.Total)
	require.Equal(t, "BATCH-0", page1.Attestations[0].BatchId)

	page2, err := qs.Attestations(f.ctx, &types.QueryAttestationsRequest{Pagination: &query.PageRequest{Key: page1.Pagination.NextKey}})
	require.NoError(t, err)
	require.Len(t, page2.Attestations, 2)
	require.Equal(t, "BATCH-4", page2.Attestations[1].BatchId)
}

func TestQuerySentAttestations(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)
	for _, id := range []string{"BATCH-A", "BATCH-B"} {
		f.addBatch(id, herbtracetypes.StatusDelivered)
		_, err := f.keeper.SendAttestation(f.ctx, f.creator, testChannel, id, f.timeout())
		require.NoError(t, err)
	}

	res, err := qs.SentAttestation(f.ctx, &types.QuerySentAttestationRequest{ChannelId: testChannel, BatchId: "BATCH-B"})
	require.NoError(t, err)
	require.Equal(t, types.SentStatusPending, res.SentAttestation.Status)
	require.Equal(t, uint64(2), res.SentAttestation.Sequence)

	_, err = qs.SentAttestation(f.ctx, &types.QuerySentAttestationRequest{ChannelId: testChannel, BatchId: "BATCH-C"})
	require.Equal(t, codes.NotFound, status.Code(err))

	all, err := qs.SentAttestations(f.ctx, &types.QuerySentAttestationsRequest{})
	require.NoError(t, err)
	require.Len(t, all.SentAttestations, 2)
}

func TestQueryNilRequests(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.Attestation(f.ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = qs.Attestations(f.ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = qs.SentAttestation(f.ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = qs.SentAttestations(f.ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = qs.Attestations(f.ctx, &types.QueryAttestationsRequest{Pagination: &query.PageRequest{Key: []byte("x"), Offset: 1}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
