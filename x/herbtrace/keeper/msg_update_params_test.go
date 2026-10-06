package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestMsgUpdateParams(t *testing.T) {
	tests := []struct {
		name   string
		input  func(f *fixture) *types.MsgUpdateParams
		err    error
		errMsg string
	}{
		{
			name: "invalid authority address",
			input: func(*fixture) *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: "invalid", Params: types.DefaultParams()}
			},
			errMsg: "invalid authority",
		},
		{
			name: "wrong authority",
			input: func(f *fixture) *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: f.farmer, Params: types.DefaultParams()}
			},
			err: types.ErrInvalidSigner,
		},
		{
			name: "zero max certifications",
			input: func(f *fixture) *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: f.authority, Params: types.Params{}}
			},
			err: types.ErrInvalidParams,
		},
		{
			name: "invalid certifier",
			input: func(f *fixture) *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: f.authority, Params: types.NewParams([]string{"bad"}, 10)}
			},
			err: types.ErrInvalidParams,
		},
		{
			name: "duplicate certifier",
			input: func(f *fixture) *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: f.authority, Params: types.NewParams([]string{f.certifier, f.certifier}, 10)}
			},
			err: types.ErrInvalidParams,
		},
		{
			name: "valid",
			input: func(f *fixture) *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: f.authority, Params: types.NewParams([]string{f.certifier, f.stranger}, 5)}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			before, err := f.keeper.Params.Get(f.ctx)
			require.NoError(t, err)

			msg := tc.input(f)
			_, err = f.msgServer.UpdateParams(f.ctx, msg)

			got, gerr := f.keeper.Params.Get(f.ctx)
			require.NoError(t, gerr)
			switch {
			case tc.err != nil:
				require.ErrorIs(t, err, tc.err)
				require.Equal(t, before, got)
			case tc.errMsg != "":
				require.ErrorContains(t, err, tc.errMsg)
				require.Equal(t, before, got)
			default:
				require.NoError(t, err)
				require.Equal(t, msg.Params, got)
			}
		})
	}
}
