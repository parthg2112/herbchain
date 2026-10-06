package keeper_test

import (
	"testing"

	"cosmossdk.io/collections"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/attest/keeper"
	"github.com/parthg2112/herbchain/x/attest/types"
	herbtracetypes "github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestSendAttestationStatusGating(t *testing.T) {
	wire := map[herbtracetypes.BatchStatus]string{
		herbtracetypes.StatusTested:    types.StatusTested,
		herbtracetypes.StatusPackaged:  types.StatusPackaged,
		herbtracetypes.StatusShipped:   types.StatusShipped,
		herbtracetypes.StatusDelivered: types.StatusDelivered,
	}

	for value := range int32(len(herbtracetypes.BatchStatus_name)) {
		status := herbtracetypes.BatchStatus(value)
		t.Run(status.String(), func(t *testing.T) {
			f := initFixture(t)
			srv := keeper.NewMsgServerImpl(f.keeper)
			f.addBatch("BATCH-1", status)

			res, err := srv.SendAttestation(f.ctx, types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-1", f.timeout()))
			want, ok := wire[status]
			if !ok {
				require.ErrorIs(t, err, types.ErrBatchNotAttestable)
				require.Empty(t, f.sender.packets)
				return
			}
			require.NoError(t, err)
			require.Equal(t, uint64(1), res.Sequence)

			require.Len(t, f.sender.packets, 1)
			p := f.sender.packets[0]
			require.Equal(t, types.PortID, p.port)
			require.Equal(t, testChannel, p.channel)
			require.True(t, p.timeoutHeight.IsZero())
			require.Equal(t, f.timeout(), p.timeout)

			var envelope types.AttestPacketData
			require.NoError(t, envelope.Unmarshal(p.data))
			require.Equal(t, types.AttestationPacketData{
				BatchId:       "BATCH-1",
				HerbType:      "ashwagandha",
				Farmer:        f.farmer,
				Status:        want,
				LabReportHash: testHash,
				SourceHeight:  testHeight,
			}, *envelope.GetAttestationPacket())

			sent, err := f.keeper.SentAttestations.Get(f.ctx, collections.Join(testChannel, "BATCH-1"))
			require.NoError(t, err)
			require.Equal(t, types.SentAttestation{
				ChannelId:     testChannel,
				BatchId:       "BATCH-1",
				Sequence:      1,
				Status:        types.SentStatusPending,
				Sender:        f.creator,
				SentHeight:    testHeight,
				UpdatedHeight: testHeight,
			}, sent)
		})
	}
}

func TestSendAttestationValidation(t *testing.T) {
	f := initFixture(t)
	srv := keeper.NewMsgServerImpl(f.keeper)
	f.addBatch("BATCH-1", herbtracetypes.StatusTested)
	bad := f.addBatch("BATCH-BAD", herbtracetypes.StatusTested)
	bad.LabReportHash = ""
	f.herbtrace.batches["BATCH-BAD"] = bad

	tests := []struct {
		name string
		msg  *types.MsgSendAttestation
		err  error
	}{
		{"invalid creator", types.NewMsgSendAttestation("nope", testChannel, "BATCH-1", f.timeout()), sdkerrors.ErrInvalidAddress},
		{"invalid channel", types.NewMsgSendAttestation(f.creator, "ch", "BATCH-1", f.timeout()), types.ErrInvalidRequest},
		{"invalid batch id", types.NewMsgSendAttestation(f.creator, testChannel, "bad id", f.timeout()), types.ErrInvalidRequest},
		{"zero timeout", types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-1", 0), types.ErrInvalidPacketTimeout},
		{"elapsed timeout", types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-1", uint64(testTime.UnixNano())), types.ErrInvalidPacketTimeout},
		{"batch not found", types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-404", f.timeout()), herbtracetypes.ErrBatchNotFound},
		{"invalid batch data", types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-BAD", f.timeout()), types.ErrInvalidPacket},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.SendAttestation(f.ctx, tc.msg)
			require.ErrorIs(t, err, tc.err)
		})
	}
	require.Empty(t, f.sender.packets)
}

func TestSendAttestationResendPolicy(t *testing.T) {
	f := initFixture(t)
	srv := keeper.NewMsgServerImpl(f.keeper)
	f.addBatch("BATCH-1", herbtracetypes.StatusShipped)
	msg := types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-1", f.timeout())
	key := collections.Join(testChannel, "BATCH-1")

	_, err := srv.SendAttestation(f.ctx, msg)
	require.NoError(t, err)

	_, err = srv.SendAttestation(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrAttestationPending)

	// Another channel is an independent destination.
	_, err = srv.SendAttestation(f.ctx, types.NewMsgSendAttestation(f.creator, "channel-1", "BATCH-1", f.timeout()))
	require.NoError(t, err)

	for _, settled := range []types.SentStatus{types.SentStatusAcked, types.SentStatusErrored, types.SentStatusTimedOut} {
		sent, err := f.keeper.SentAttestations.Get(f.ctx, key)
		require.NoError(t, err)
		sent.Status = settled
		require.NoError(t, f.keeper.SentAttestations.Set(f.ctx, key, sent))

		res, err := srv.SendAttestation(f.ctx, msg)
		require.NoError(t, err, settled.String())

		sent, err = f.keeper.SentAttestations.Get(f.ctx, key)
		require.NoError(t, err)
		require.Equal(t, types.SentStatusPending, sent.Status)
		require.Equal(t, res.Sequence, sent.Sequence)
	}
}

func TestSendAttestationSendFailure(t *testing.T) {
	f := initFixture(t)
	srv := keeper.NewMsgServerImpl(f.keeper)
	f.addBatch("BATCH-1", herbtracetypes.StatusTested)
	f.sender.err = errChannelNotFound

	_, err := srv.SendAttestation(f.ctx, types.NewMsgSendAttestation(f.creator, testChannel, "BATCH-1", f.timeout()))
	require.ErrorIs(t, err, errChannelNotFound)

	has, err := f.keeper.SentAttestations.Has(f.ctx, collections.Join(testChannel, "BATCH-1"))
	require.NoError(t, err)
	require.False(t, has)
}

func TestSendAttestationWithoutIBC(t *testing.T) {
	f := initFixture(t)
	f.addBatch("BATCH-1", herbtracetypes.StatusTested)
	k := f.keeper.WithPacketSender(nil)

	_, err := k.SendAttestation(f.ctx, f.creator, testChannel, "BATCH-1", f.timeout())
	require.ErrorIs(t, err, types.ErrIBCUnavailable)
}
