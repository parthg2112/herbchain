package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestGenesisDefault(t *testing.T) {
	f := initFixture(t)
	require.NoError(t, f.keeper.InitGenesis(f.ctx, *types.DefaultGenesis()))
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultGenesis(), got)
}

func TestGenesisRoundTrip(t *testing.T) {
	src := initFixture(t)
	src.createAt(t, "GEN-A", types.StatusDelivered)
	src.createAt(t, "GEN-B", types.StatusRejected)
	src.createAt(t, "GEN-C", types.StatusProcessed)
	src.createAt(t, "GEN-D", types.StatusHarvested)
	_, err := src.msgServer.TransferCustody(src.ctx, &types.MsgTransferCustody{
		Owner: src.farmer, BatchId: "GEN-C", NewOwner: src.processor, Location: "Delhi",
	})
	require.NoError(t, err)

	exported, err := src.keeper.ExportGenesis(src.ctx)
	require.NoError(t, err)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.Batches, 4)
	require.Len(t, exported.HistorySeqs, 4)
	require.Len(t, exported.History, 6+3+3+1)

	dst := initFixture(t)
	require.NoError(t, dst.keeper.InitGenesis(dst.ctx, *exported))
	reexported, err := dst.keeper.ExportGenesis(dst.ctx)
	require.NoError(t, err)
	require.Equal(t, exported, reexported)

	// Indexes are not exported; they must be rebuilt identically by InitGenesis.
	for _, owner := range []string{src.farmer, src.processor, src.stranger} {
		require.Equal(t, src.idsByOwner(t, owner), dst.idsByOwner(t, owner), "owner %s", owner)
	}
	for _, s := range reachableStatuses {
		require.Equal(t, src.idsByStatus(t, s), dst.idsByStatus(t, s), "status %s", s)
	}
	require.Equal(t, []string{"GEN-C"}, dst.idsByOwner(t, dst.processor))

	// Sequences continue after import.
	_, err = dst.msgServer.UpdateStatus(dst.ctx, &types.MsgUpdateStatus{
		Owner: dst.farmer, BatchId: "GEN-D", Status: types.StatusProcessed,
	})
	require.NoError(t, err)
	hist := dst.history(t, "GEN-D")
	require.Len(t, hist, 2)
	require.Equal(t, uint64(1), hist[1].Seq)
}

func TestInitGenesisRejectsBadOwner(t *testing.T) {
	f := initFixture(t)
	gs := types.DefaultGenesis()
	gs.Batches = []types.Batch{{Id: "BAD-1", Owner: "not-an-address"}}
	require.ErrorIs(t, f.keeper.InitGenesis(f.ctx, *gs), types.ErrInvalidAddress)
}
