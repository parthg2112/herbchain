package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/parthg2112/herbchain/x/attest/types"
)

func (k msgServer) SendAttestation(ctx context.Context, msg *types.MsgSendAttestation) (*types.MsgSendAttestationResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address: %s", err)
	}

	seq, err := k.Keeper.SendAttestation(ctx, msg.Creator, msg.ChannelId, msg.BatchId, msg.TimeoutTimestamp)
	if err != nil {
		return nil, err
	}

	return &types.MsgSendAttestationResponse{Sequence: seq}, nil
}
