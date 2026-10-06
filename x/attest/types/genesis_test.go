package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/attest/types"
)

const (
	hash   = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	farmer = "herb1trjvk0a7m5ta4ms2xayet88d9pe99dmynz25kw"
)

func validAttestation() types.Attestation {
	return types.Attestation{
		ChannelId: "channel-0", SourcePort: types.PortID, SourceChannel: "channel-1", BatchId: "BATCH-1",
		HerbType: "tulsi", Farmer: farmer, Status: types.StatusTested, LabReportHash: hash,
		SourceHeight: 5, Sequence: 1,
	}
}

func validSent() types.SentAttestation {
	return types.SentAttestation{ChannelId: "channel-0", BatchId: "BATCH-1", Sequence: 1, Status: types.SentStatusPending}
}

func TestGenesisState_Validate(t *testing.T) {
	withAtt := func(fn func(*types.Attestation)) *types.GenesisState {
		a := validAttestation()
		fn(&a)
		gs := types.DefaultGenesis()
		gs.Attestations = []types.Attestation{a}
		return gs
	}
	withSent := func(fn func(*types.SentAttestation)) *types.GenesisState {
		s := validSent()
		fn(&s)
		gs := types.DefaultGenesis()
		gs.SentAttestations = []types.SentAttestation{s}
		return gs
	}

	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{"default is valid", types.DefaultGenesis(), true},
		{"populated", &types.GenesisState{
			PortId:           types.PortID,
			Attestations:     []types.Attestation{validAttestation()},
			SentAttestations: []types.SentAttestation{validSent()},
		}, true},
		{"wrong port", &types.GenesisState{PortId: "transfer"}, false},
		{"attestation bad channel", withAtt(func(a *types.Attestation) { a.ChannelId = "x" }), false},
		{"attestation bad packet", withAtt(func(a *types.Attestation) { a.Status = "REJECTED" }), false},
		{"attestation zero sequence", withAtt(func(a *types.Attestation) { a.Sequence = 0 }), false},
		{"duplicate attestation", &types.GenesisState{
			PortId:       types.PortID,
			Attestations: []types.Attestation{validAttestation(), validAttestation()},
		}, false},
		{"sent bad channel", withSent(func(s *types.SentAttestation) { s.ChannelId = "x" }), false},
		{"sent bad batch", withSent(func(s *types.SentAttestation) { s.BatchId = "x" }), false},
		{"sent zero sequence", withSent(func(s *types.SentAttestation) { s.Sequence = 0 }), false},
		{"sent acked beyond sequence", withSent(func(s *types.SentAttestation) { s.AckedSequence = 2 }), false},
		{"sent unspecified status", withSent(func(s *types.SentAttestation) { s.Status = types.SentStatusUnspecified }), false},
		{"sent unknown status", withSent(func(s *types.SentAttestation) { s.Status = 99 }), false},
		{"duplicate sent", &types.GenesisState{
			PortId:           types.PortID,
			SentAttestations: []types.SentAttestation{validSent(), validSent()},
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPacketValidateBasic(t *testing.T) {
	for _, status := range []string{types.StatusTested, types.StatusPackaged, types.StatusShipped, types.StatusDelivered} {
		p := validAttestation().PacketData()
		p.Status = status
		require.NoError(t, p.ValidateBasic(), status)
	}

	p := validAttestation().PacketData()
	p.LabReportHash = "9F86D081884C7D659A2FEAA0C55AD015A3BF4F1B2B0B822CD15D6C15B0F00A08"
	require.NoError(t, p.ValidateBasic(), "upper-case hex")

	p.LabReportHash = "zz" + hash[2:]
	require.ErrorIs(t, p.ValidateBasic(), types.ErrInvalidPacket)
}
