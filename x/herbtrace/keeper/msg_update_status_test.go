package keeper_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

var (
	reachableStatuses = []types.BatchStatus{
		types.StatusHarvested, types.StatusProcessed, types.StatusTested, types.StatusPackaged,
		types.StatusShipped, types.StatusDelivered, types.StatusRejected,
	}
	targetStatuses = append([]types.BatchStatus{types.StatusUnspecified}, append(reachableStatuses, 99)...)
)

func TestUpdateStatusFullLifecycle(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ASH-001", types.StatusTested)

	for _, s := range []types.BatchStatus{types.StatusPackaged, types.StatusShipped, types.StatusDelivered} {
		f.freshEvents()
		before, err := f.keeper.GetBatch(f.ctx, "ASH-001")
		require.NoError(t, err)
		loc := "loc-" + s.String()
		_, err = f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{
			Owner: f.farmer, BatchId: "ASH-001", Status: s, Location: loc, Remark: "step",
		})
		require.NoError(t, err)

		b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
		require.NoError(t, err)
		require.Equal(t, s, b.Status)
		require.Equal(t, loc, b.Location)
		require.Equal(t, []string{"ASH-001"}, f.idsByStatus(t, s))
		require.Empty(t, f.idsByStatus(t, before.Status), "old status index entry removed")
		requireTypedEvent(t, f.ctx, &types.EventStatusUpdated{
			BatchId: "ASH-001", Actor: f.farmer, FromStatus: before.Status, ToStatus: s, Location: loc,
		})
	}

	hist := f.history(t, "ASH-001")
	require.Len(t, hist, 6)
	for i, e := range hist {
		require.Equal(t, uint64(i), e.Seq)
	}
	last := hist[5]
	require.Equal(t, types.ActionStatusUpdated, last.Action)
	require.Equal(t, types.StatusShipped, last.FromStatus)
	require.Equal(t, types.StatusDelivered, last.ToStatus)
	require.Equal(t, "step", last.Note)
}

// TestUpdateStatusEveryTransition drives a batch into every reachable status and attempts every target.
func TestUpdateStatusEveryTransition(t *testing.T) {
	f := initFixture(t)
	for _, from := range reachableStatuses {
		for _, to := range targetStatuses {
			id := fmt.Sprintf("B-%d-%d", from, to)
			t.Run(fmt.Sprintf("%s->%s", from, to), func(t *testing.T) {
				f.createAt(t, id, from)
				_, err := f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{Owner: f.farmer, BatchId: id, Status: to})

				switch {
				case from.IsTerminal():
					require.ErrorIs(t, err, types.ErrTerminalStatus)
				case !types.IsOwnerDrivenStatus(to):
					require.ErrorIs(t, err, types.ErrInvalidTransition)
				case types.ValidateTransition(from, to) == nil:
					require.NoError(t, err)
					return
				default:
					require.ErrorIs(t, err, types.ErrInvalidTransition)
				}
				b, gerr := f.keeper.GetBatch(f.ctx, id)
				require.NoError(t, gerr)
				require.Equal(t, from, b.Status, "status unchanged on failure")
			})
		}
	}
}

func TestUpdateStatusErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(f *fixture, m *types.MsgUpdateStatus)
		err    error
	}{
		{"wrong signer", func(f *fixture, m *types.MsgUpdateStatus) { m.Owner = f.stranger }, types.ErrUnauthorized},
		{"certifier is not owner", func(f *fixture, m *types.MsgUpdateStatus) { m.Owner = f.certifier }, types.ErrUnauthorized},
		{"invalid signer address", func(_ *fixture, m *types.MsgUpdateStatus) { m.Owner = "bad" }, types.ErrInvalidAddress},
		{"batch not found", func(_ *fixture, m *types.MsgUpdateStatus) { m.BatchId = "NOPE-1" }, types.ErrBatchNotFound},
		{"skipped transition", func(_ *fixture, m *types.MsgUpdateStatus) { m.Status = types.StatusPackaged }, types.ErrInvalidTransition},
		{"lab-only target", func(_ *fixture, m *types.MsgUpdateStatus) { m.Status = types.StatusTested }, types.ErrInvalidTransition},
		{"location too long", func(_ *fixture, m *types.MsgUpdateStatus) { m.Location = strings.Repeat("l", 129) }, types.ErrInvalidBatch},
		{"remark too long", func(_ *fixture, m *types.MsgUpdateStatus) { m.Remark = strings.Repeat("r", 257) }, types.ErrInvalidBatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.createAt(t, "ASH-001", types.StatusHarvested)
			msg := &types.MsgUpdateStatus{Owner: f.farmer, BatchId: "ASH-001", Status: types.StatusProcessed}
			tc.mutate(f, msg)
			_, err := f.msgServer.UpdateStatus(f.ctx, msg)
			require.ErrorIs(t, err, tc.err)
			require.Len(t, f.history(t, "ASH-001"), 1, "no history appended on failure")
		})
	}
}

func TestUpdateStatusEmptyLocationKeepsCurrent(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ASH-001", types.StatusHarvested)
	_, err := f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{
		Owner: f.farmer, BatchId: "ASH-001", Status: types.StatusProcessed,
	})
	require.NoError(t, err)
	b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, "Jaipur farm", b.Location)
}

func TestUpdateStatusRecordsBlockContext(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ASH-001", types.StatusHarvested)
	f.ctx = f.ctx.WithBlockHeight(testHeight + 5).WithBlockTime(testBlockTime.Add(time.Minute))
	_, err := f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{
		Owner: f.farmer, BatchId: "ASH-001", Status: types.StatusProcessed,
	})
	require.NoError(t, err)

	b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, testHeight, b.CreatedHeight)
	require.Equal(t, testHeight+5, b.UpdatedHeight)
	e := f.history(t, "ASH-001")[1]
	require.Equal(t, testHeight+5, e.Height)
	require.Equal(t, testBlockTime.Unix()+60, e.Time)
}
