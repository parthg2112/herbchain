package types_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

// allStatuses includes UNSPECIFIED and an out-of-range value to exercise the guards.
var allStatuses = []types.BatchStatus{
	types.StatusUnspecified, types.StatusHarvested, types.StatusProcessed, types.StatusTested,
	types.StatusPackaged, types.StatusShipped, types.StatusDelivered, types.StatusRejected, types.BatchStatus(99),
}

func TestValidateTransitionExhaustive(t *testing.T) {
	legal := map[[2]types.BatchStatus]bool{
		{types.StatusHarvested, types.StatusProcessed}: true,
		{types.StatusProcessed, types.StatusTested}:    true,
		{types.StatusProcessed, types.StatusRejected}:  true,
		{types.StatusTested, types.StatusPackaged}:     true,
		{types.StatusPackaged, types.StatusShipped}:    true,
		{types.StatusShipped, types.StatusDelivered}:   true,
	}
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			t.Run(fmt.Sprintf("%s->%s", from, to), func(t *testing.T) {
				err := types.ValidateTransition(from, to)
				switch {
				case legal[[2]types.BatchStatus{from, to}]:
					require.NoError(t, err)
				case from == types.StatusDelivered || from == types.StatusRejected:
					require.ErrorIs(t, err, types.ErrTerminalStatus)
				default:
					require.ErrorIs(t, err, types.ErrInvalidTransition)
				}
			})
		}
	}
}

func TestStatusClassifiers(t *testing.T) {
	tests := []struct {
		s                             types.BatchStatus
		valid, terminal, owner, isLab bool
	}{
		{types.StatusUnspecified, false, false, false, false},
		{types.StatusHarvested, true, false, false, false},
		{types.StatusProcessed, true, false, true, false},
		{types.StatusTested, true, false, false, true},
		{types.StatusPackaged, true, false, true, false},
		{types.StatusShipped, true, false, true, false},
		{types.StatusDelivered, true, true, true, false},
		{types.StatusRejected, true, true, false, true},
		{types.BatchStatus(99), false, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.s.String(), func(t *testing.T) {
			require.Equal(t, tc.valid, types.IsValidStatus(tc.s))
			require.Equal(t, tc.terminal, tc.s.IsTerminal())
			require.Equal(t, tc.owner, types.IsOwnerDrivenStatus(tc.s))
			require.Equal(t, tc.isLab, types.IsLabStatus(tc.s))
		})
	}
}
