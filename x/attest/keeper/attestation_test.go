package keeper_test

import (
	"testing"

	"cosmossdk.io/collections"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/attest/types"
	herbtracetypes "github.com/parthg2112/herbchain/x/herbtrace/types"
)

func (f *fixture) packetData(batchID string) types.AttestationPacketData {
	return types.AttestationPacketData{
		BatchId:       batchID,
		HerbType:      "tulsi",
		Farmer:        f.farmer,
		Status:        types.StatusTested,
		LabReportHash: testHash,
		SourceHeight:  7,
	}
}

func recvPacket(destChannel string, seq uint64) channeltypes.Packet {
	return channeltypes.Packet{
		Sequence:           seq,
		SourcePort:         types.PortID,
		SourceChannel:      "channel-9",
		DestinationPort:    types.PortID,
		DestinationChannel: destChannel,
	}
}

func TestOnRecvAttestationPacket(t *testing.T) {
	f := initFixture(t)
	data := f.packetData("BATCH-1")

	ack, err := f.keeper.OnRecvAttestationPacket(f.ctx, recvPacket(testChannel, 3), data)
	require.NoError(t, err)
	require.True(t, ack.Received)

	got, err := f.keeper.Attestations.Get(f.ctx, collections.Join(testChannel, "BATCH-1"))
	require.NoError(t, err)
	require.Equal(t, types.Attestation{
		ChannelId:      testChannel,
		SourcePort:     types.PortID,
		SourceChannel:  "channel-9",
		BatchId:        "BATCH-1",
		HerbType:       "tulsi",
		Farmer:         f.farmer,
		Status:         types.StatusTested,
		LabReportHash:  testHash,
		SourceHeight:   7,
		Sequence:       3,
		ReceivedHeight: testHeight,
		ReceivedTime:   testTime.Unix(),
	}, got)

	_, err = f.keeper.OnRecvAttestationPacket(f.ctx, recvPacket(testChannel, 4), data)
	require.ErrorIs(t, err, types.ErrDuplicateAttestation)

	// The same batch from a different counterparty channel is a separate attestation.
	_, err = f.keeper.OnRecvAttestationPacket(f.ctx, recvPacket("channel-1", 1), data)
	require.NoError(t, err)
}

func TestOnRecvAttestationPacketInvalid(t *testing.T) {
	f := initFixture(t)
	mutate := map[string]func(*types.AttestationPacketData){
		"bad batch id":    func(d *types.AttestationPacketData) { d.BatchId = "b" },
		"empty herb type": func(d *types.AttestationPacketData) { d.HerbType = "" },
		"long herb type":  func(d *types.AttestationPacketData) { d.HerbType = string(make([]byte, 65)) },
		"bad farmer":      func(d *types.AttestationPacketData) { d.Farmer = "farmer" },
		"long farmer":     func(d *types.AttestationPacketData) { d.Farmer = string(make([]byte, 129)) },
		"bad status":      func(d *types.AttestationPacketData) { d.Status = "HARVESTED" },
		"short hash":      func(d *types.AttestationPacketData) { d.LabReportHash = "abc" },
		"zero height":     func(d *types.AttestationPacketData) { d.SourceHeight = 0 },
	}
	for name, fn := range mutate {
		t.Run(name, func(t *testing.T) {
			data := f.packetData("BATCH-1")
			fn(&data)
			_, err := f.keeper.OnRecvAttestationPacket(f.ctx, recvPacket(testChannel, 1), data)
			require.ErrorIs(t, err, types.ErrInvalidPacket)
		})
	}
	has, err := f.keeper.Attestations.Has(f.ctx, collections.Join(testChannel, "BATCH-1"))
	require.NoError(t, err)
	require.False(t, has)
}

// sendOne sends BATCH-1 on testChannel and returns the outgoing packet as seen by the callbacks.
func (f *fixture) sendOne(t *testing.T) (channeltypes.Packet, types.AttestationPacketData) {
	t.Helper()
	f.addBatch("BATCH-1", herbtracetypes.StatusTested)
	seq, err := f.keeper.SendAttestation(f.ctx, f.creator, testChannel, "BATCH-1", f.timeout())
	require.NoError(t, err)
	packet := channeltypes.Packet{Sequence: seq, SourcePort: types.PortID, SourceChannel: testChannel}
	return packet, types.AttestationPacketData{BatchId: "BATCH-1"}
}

func (f *fixture) sent(t *testing.T) types.SentAttestation {
	t.Helper()
	s, err := f.keeper.SentAttestations.Get(f.ctx, collections.Join(testChannel, "BATCH-1"))
	require.NoError(t, err)
	return s
}

func TestOnAcknowledgementAttestationPacket(t *testing.T) {
	tests := []struct {
		name    string
		ack     channeltypes.Acknowledgement
		status  types.SentStatus
		errText string
	}{
		{"success", channeltypes.NewResultAcknowledgement([]byte(`{"received":true}`)), types.SentStatusAcked, ""},
		{"error ack", channeltypes.NewErrorAcknowledgementWithCodespace(types.ErrDuplicateAttestation),
			types.SentStatusErrored, "ABCI error: attest/1102: error handling packet: see events for details"},
		{"not received", channeltypes.NewResultAcknowledgement([]byte(`{"received":false}`)),
			types.SentStatusErrored, "counterparty did not record attestation"},
		{"malformed result", channeltypes.NewResultAcknowledgement([]byte(`not json`)),
			types.SentStatusErrored, "malformed acknowledgement result"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			packet, data := f.sendOne(t)
			ctx := f.ctx.WithBlockHeight(testHeight + 5)

			require.NoError(t, f.keeper.OnAcknowledgementAttestationPacket(ctx, packet, data, tc.ack))
			s := f.sent(t)
			require.Equal(t, tc.status, s.Status)
			require.Equal(t, tc.errText, s.Error)
			require.Equal(t, int64(testHeight+5), s.UpdatedHeight)
			require.Equal(t, int64(testHeight), s.SentHeight)
		})
	}
}

func TestOnAcknowledgementAttestationPacketEdgeCases(t *testing.T) {
	f := initFixture(t)
	packet, data := f.sendOne(t)

	err := f.keeper.OnAcknowledgementAttestationPacket(f.ctx, packet, data, channeltypes.Acknowledgement{})
	require.ErrorIs(t, err, types.ErrInvalidAck)

	// A callback for another sequence must not overwrite the in-flight record.
	stale := packet
	stale.Sequence++
	ok := channeltypes.NewResultAcknowledgement([]byte(`{"received":true}`))
	require.NoError(t, f.keeper.OnAcknowledgementAttestationPacket(f.ctx, stale, data, ok))
	require.Equal(t, types.SentStatusPending, f.sent(t).Status)

	unknown := types.AttestationPacketData{BatchId: "BATCH-404"}
	require.NoError(t, f.keeper.OnAcknowledgementAttestationPacket(f.ctx, packet, unknown, ok))
}

func TestAckedSequenceSurvivesResend(t *testing.T) {
	f := initFixture(t)
	packet, data := f.sendOne(t)
	ok := channeltypes.NewResultAcknowledgement([]byte(`{"received":true}`))
	require.NoError(t, f.keeper.OnAcknowledgementAttestationPacket(f.ctx, packet, data, ok))
	require.Equal(t, packet.Sequence, f.sent(t).AckedSequence)

	seq, err := f.keeper.SendAttestation(f.ctx, f.creator, testChannel, "BATCH-1", f.timeout())
	require.NoError(t, err)
	packet.Sequence = seq
	dup := channeltypes.NewErrorAcknowledgementWithCodespace(types.ErrDuplicateAttestation)
	require.NoError(t, f.keeper.OnAcknowledgementAttestationPacket(f.ctx, packet, data, dup))

	s := f.sent(t)
	require.Equal(t, types.SentStatusErrored, s.Status)
	require.Equal(t, seq, s.Sequence)
	require.Equal(t, seq-1, s.AckedSequence)
}

func TestOnTimeoutAttestationPacket(t *testing.T) {
	f := initFixture(t)
	packet, data := f.sendOne(t)

	require.NoError(t, f.keeper.OnTimeoutAttestationPacket(f.ctx, packet, data))
	s := f.sent(t)
	require.Equal(t, types.SentStatusTimedOut, s.Status)
	require.Equal(t, "packet timed out", s.Error)
}
