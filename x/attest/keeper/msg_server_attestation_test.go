package keeper_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/parthg2112/herbchain/x/attest/keeper"
	"github.com/parthg2112/herbchain/x/attest/types"
)

func TestMsgServerSendAttestation(t *testing.T) {
	f := initFixture(t)
	srv := keeper.NewMsgServerImpl(f.keeper)
	creator, err := f.addressCodec.BytesToString([]byte("signerAddr__________________"))
	require.NoError(t, err)

	tests := []struct {
		name string
		msg  types.MsgSendAttestation
		err  error
	}{
		{
			name: "invalid address",
			msg: types.MsgSendAttestation{
				Creator:          "invalid address",
				Port:             "port",
				ChannelID:        "channel-0",
				TimeoutTimestamp: 100,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid port",
			msg: types.MsgSendAttestation{
				Creator:          creator,
				Port:             "",
				ChannelID:        "channel-0",
				TimeoutTimestamp: 100,
			},
			err: sdkerrors.ErrInvalidRequest,
		}, {
			name: "invalid channel",
			msg: types.MsgSendAttestation{
				Creator:          creator,
				Port:             "port",
				ChannelID:        "",
				TimeoutTimestamp: 100,
			},
			err: sdkerrors.ErrInvalidRequest,
		}, {
			name: "invalid timeout",
			msg: types.MsgSendAttestation{
				Creator:          creator,
				Port:             "port",
				ChannelID:        "channel-0",
				TimeoutTimestamp: 0,
			},
			err: sdkerrors.ErrInvalidRequest,
		}, {
			name: "valid message",
			msg: types.MsgSendAttestation{
				Creator:          creator,
				Port:             "port",
				ChannelID:        "channel-0",
				TimeoutTimestamp: 100,
			},
			err: errors.New("channel not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = srv.SendAttestation(f.ctx, &tt.msg)
			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
