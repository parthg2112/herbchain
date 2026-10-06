package keeper_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestRecordLabResult(t *testing.T) {
	tests := []struct {
		passed bool
		want   types.BatchStatus
	}{
		{true, types.StatusTested},
		{false, types.StatusRejected},
	}
	for _, tc := range tests {
		t.Run(tc.want.String(), func(t *testing.T) {
			f := initFixture(t)
			f.createAt(t, "ASH-001", types.StatusProcessed)
			f.freshEvents()

			res, err := f.msgServer.RecordLabResult(f.ctx, &types.MsgRecordLabResult{
				Certifier: f.certifier, BatchId: "ASH-001", Passed: tc.passed, ReportHash: reportHash, Remark: "lab-7",
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, res.Status)

			b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
			require.NoError(t, err)
			require.Equal(t, tc.want, b.Status)
			require.Equal(t, reportHash, b.LabReportHash)
			require.Equal(t, f.farmer, b.Owner, "lab result does not change custody")
			require.Equal(t, []string{"ASH-001"}, f.idsByStatus(t, tc.want))
			require.Empty(t, f.idsByStatus(t, types.StatusProcessed))

			last := f.history(t, "ASH-001")[2]
			require.Equal(t, types.ActionLabResult, last.Action)
			require.Equal(t, f.certifier, last.Actor)
			require.Equal(t, types.StatusProcessed, last.FromStatus)
			require.Equal(t, tc.want, last.ToStatus)
			require.Equal(t, "lab-7", last.Note)
			requireTypedEvent(t, f.ctx, &types.EventLabResultRecorded{
				BatchId: "ASH-001", Certifier: f.certifier, Passed: tc.passed, ReportHash: reportHash, ToStatus: tc.want,
			})
		})
	}
}

func TestRecordLabResultErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(f *fixture, m *types.MsgRecordLabResult)
		err    error
	}{
		{"non-certifier", func(f *fixture, m *types.MsgRecordLabResult) { m.Certifier = f.stranger }, types.ErrUnauthorized},
		{"owner is not certifier", func(f *fixture, m *types.MsgRecordLabResult) { m.Certifier = f.farmer }, types.ErrUnauthorized},
		{"invalid certifier address", func(_ *fixture, m *types.MsgRecordLabResult) { m.Certifier = "bad" }, types.ErrInvalidAddress},
		{"batch not found", func(_ *fixture, m *types.MsgRecordLabResult) { m.BatchId = "NOPE-1" }, types.ErrBatchNotFound},
		{"hash empty", func(_ *fixture, m *types.MsgRecordLabResult) { m.ReportHash = "" }, types.ErrInvalidReportHash},
		{"hash short", func(_ *fixture, m *types.MsgRecordLabResult) { m.ReportHash = reportHash[:63] }, types.ErrInvalidReportHash},
		{"hash uppercase", func(_ *fixture, m *types.MsgRecordLabResult) { m.ReportHash = strings.ToUpper(reportHash) }, types.ErrInvalidReportHash},
		{"hash non-hex", func(_ *fixture, m *types.MsgRecordLabResult) { m.ReportHash = "g" + reportHash[1:] }, types.ErrInvalidReportHash},
		{"remark too long", func(_ *fixture, m *types.MsgRecordLabResult) { m.Remark = strings.Repeat("r", 257) }, types.ErrInvalidBatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.createAt(t, "ASH-001", types.StatusProcessed)
			msg := &types.MsgRecordLabResult{Certifier: f.certifier, BatchId: "ASH-001", Passed: true, ReportHash: reportHash}
			tc.mutate(f, msg)
			_, err := f.msgServer.RecordLabResult(f.ctx, msg)
			require.ErrorIs(t, err, tc.err)

			b, gerr := f.keeper.GetBatch(f.ctx, "ASH-001")
			require.NoError(t, gerr)
			require.Equal(t, types.StatusProcessed, b.Status)
			require.Empty(t, b.LabReportHash)
		})
	}
}

// TestRecordLabResultFromEveryStatus checks lab results are accepted only while PROCESSED.
func TestRecordLabResultFromEveryStatus(t *testing.T) {
	f := initFixture(t)
	for _, from := range reachableStatuses {
		for i, passed := range []bool{true, false} {
			id := fmt.Sprintf("L-%d-%d", from, i)
			t.Run(id, func(t *testing.T) {
				f.createAt(t, id, from)
				_, err := f.msgServer.RecordLabResult(f.ctx, &types.MsgRecordLabResult{
					Certifier: f.certifier, BatchId: id, Passed: passed, ReportHash: reportHash,
				})
				switch {
				case from == types.StatusProcessed:
					require.NoError(t, err)
				case from.IsTerminal():
					require.ErrorIs(t, err, types.ErrTerminalStatus)
				default:
					require.ErrorIs(t, err, types.ErrInvalidTransition)
				}
			})
		}
	}
}

func TestRecordLabResultAfterCertifierRemoved(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ASH-001", types.StatusProcessed)
	_, err := f.msgServer.UpdateParams(f.ctx, &types.MsgUpdateParams{Authority: f.authority, Params: types.DefaultParams()})
	require.NoError(t, err)

	_, err = f.msgServer.RecordLabResult(f.ctx, &types.MsgRecordLabResult{
		Certifier: f.certifier, BatchId: "ASH-001", Passed: true, ReportHash: reportHash,
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}
