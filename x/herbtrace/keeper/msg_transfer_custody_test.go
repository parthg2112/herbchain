package keeper_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestTransferCustody(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ASH-001", types.StatusHarvested)
	f.createAt(t, "ASH-002", types.StatusHarvested)
	f.freshEvents()

	_, err := f.msgServer.TransferCustody(f.ctx, &types.MsgTransferCustody{
		Owner: f.farmer, BatchId: "ASH-001", NewOwner: f.processor, Location: "Delhi unit", Remark: "sold",
	})
	require.NoError(t, err)

	b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, f.processor, b.Owner)
	require.Equal(t, f.farmer, b.Farmer, "farmer is immutable")
	require.Equal(t, "Delhi unit", b.Location)
	require.Equal(t, types.StatusHarvested, b.Status)

	require.Equal(t, []string{"ASH-002"}, f.idsByOwner(t, f.farmer))
	require.Equal(t, []string{"ASH-001"}, f.idsByOwner(t, f.processor))
	require.Equal(t, []string{"ASH-001", "ASH-002"}, f.idsByStatus(t, types.StatusHarvested))

	last := f.history(t, "ASH-001")[1]
	require.Equal(t, types.ActionCustodyTransferred, last.Action)
	require.Equal(t, f.farmer, last.Actor)
	require.Equal(t, types.StatusHarvested, last.FromStatus)
	require.Equal(t, types.StatusHarvested, last.ToStatus)
	require.Equal(t, "sold", last.Note)
	requireTypedEvent(t, f.ctx, &types.EventCustodyTransferred{BatchId: "ASH-001", FromOwner: f.farmer, ToOwner: f.processor})

	// Authority follows custody.
	_, err = f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{Owner: f.farmer, BatchId: "ASH-001", Status: types.StatusProcessed})
	require.ErrorIs(t, err, types.ErrUnauthorized)
	_, err = f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{Owner: f.processor, BatchId: "ASH-001", Status: types.StatusProcessed})
	require.NoError(t, err)
	_, err = f.msgServer.TransferCustody(f.ctx, &types.MsgTransferCustody{Owner: f.farmer, BatchId: "ASH-001", NewOwner: f.stranger})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestTransferCustodyErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(f *fixture, m *types.MsgTransferCustody)
		err    error
	}{
		{"wrong signer", func(f *fixture, m *types.MsgTransferCustody) { m.Owner = f.stranger }, types.ErrUnauthorized},
		{"invalid signer", func(_ *fixture, m *types.MsgTransferCustody) { m.Owner = "bad" }, types.ErrInvalidAddress},
		{"invalid new owner", func(_ *fixture, m *types.MsgTransferCustody) { m.NewOwner = "bad" }, types.ErrInvalidAddress},
		{"empty new owner", func(_ *fixture, m *types.MsgTransferCustody) { m.NewOwner = "" }, types.ErrInvalidAddress},
		{"new owner is owner", func(f *fixture, m *types.MsgTransferCustody) { m.NewOwner = f.farmer }, types.ErrInvalidAddress},
		{"batch not found", func(_ *fixture, m *types.MsgTransferCustody) { m.BatchId = "NOPE-1" }, types.ErrBatchNotFound},
		{"location too long", func(_ *fixture, m *types.MsgTransferCustody) { m.Location = strings.Repeat("l", 129) }, types.ErrInvalidBatch},
		{"remark too long", func(_ *fixture, m *types.MsgTransferCustody) { m.Remark = strings.Repeat("r", 257) }, types.ErrInvalidBatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.createAt(t, "ASH-001", types.StatusHarvested)
			msg := &types.MsgTransferCustody{Owner: f.farmer, BatchId: "ASH-001", NewOwner: f.processor}
			tc.mutate(f, msg)
			_, err := f.msgServer.TransferCustody(f.ctx, msg)
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, []string{"ASH-001"}, f.idsByOwner(t, f.farmer))
		})
	}
}

func TestTransferCustodyTerminal(t *testing.T) {
	for _, s := range []types.BatchStatus{types.StatusDelivered, types.StatusRejected} {
		t.Run(s.String(), func(t *testing.T) {
			f := initFixture(t)
			f.createAt(t, "ASH-001", s)
			_, err := f.msgServer.TransferCustody(f.ctx, &types.MsgTransferCustody{
				Owner: f.farmer, BatchId: "ASH-001", NewOwner: f.processor,
			})
			require.ErrorIs(t, err, types.ErrTerminalStatus)
		})
	}
}
