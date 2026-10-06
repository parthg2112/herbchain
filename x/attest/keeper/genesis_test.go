package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/attest/types"
)

func TestGenesisRoundTrip(t *testing.T) {
	src := initFixture(t)
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		PortId: types.PortID,
		Attestations: []types.Attestation{
			{ChannelId: "channel-0", SourcePort: types.PortID, SourceChannel: "channel-3", BatchId: "BATCH-1",
				HerbType: "neem", Farmer: src.farmer, Status: types.StatusShipped, LabReportHash: testHash,
				SourceHeight: 9, Sequence: 1, ReceivedHeight: 11, ReceivedTime: 1_700_000_000},
		},
		SentAttestations: []types.SentAttestation{
			{ChannelId: "channel-0", BatchId: "BATCH-1", Sequence: 1, Status: types.SentStatusAcked, Sender: src.creator, SentHeight: 3, UpdatedHeight: 5, AckedSequence: 1},
			{ChannelId: "channel-1", BatchId: "BATCH-2", Sequence: 4, Status: types.SentStatusErrored, Error: "x", SentHeight: 6, UpdatedHeight: 8},
		},
	}
	require.NoError(t, genesisState.Validate())

	require.NoError(t, src.keeper.InitGenesis(src.ctx, genesisState))
	exported, err := src.keeper.ExportGenesis(src.ctx)
	require.NoError(t, err)
	require.Equal(t, genesisState, *exported)

	dst := initFixture(t)
	require.NoError(t, dst.keeper.InitGenesis(dst.ctx, *exported))
	reexported, err := dst.keeper.ExportGenesis(dst.ctx)
	require.NoError(t, err)
	require.Equal(t, exported, reexported)
}
