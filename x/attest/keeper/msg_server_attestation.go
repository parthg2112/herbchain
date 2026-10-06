package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	"github.com/parthg2112/herbchain/x/attest/types"
)

func (k msgServer) SendAttestation(ctx context.Context, msg *types.MsgSendAttestation) (*types.MsgSendAttestationResponse, error) {
	// validate incoming message
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, fmt.Sprintf("invalid address: %s", err))
	}

	if msg.Port == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet port")
	}

	if msg.ChannelID == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet channel")
	}

	if msg.TimeoutTimestamp == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet timeout")
	}

	// TODO: logic before transmitting the packet

	// Construct the packet
	var packet types.AttestationPacketData

	packet.BatchId = msg.BatchId
	packet.HerbType = msg.HerbType
	packet.Farmer = msg.Farmer
	packet.Status = msg.Status
	packet.LabReportHash = msg.LabReportHash
	packet.SourceHeight = msg.SourceHeight

	// Transmit the packet
	_, err := k.TransmitAttestationPacket(
		ctx,
		packet,
		msg.Port,
		msg.ChannelID,
		clienttypes.ZeroHeight(),
		msg.TimeoutTimestamp,
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgSendAttestationResponse{}, nil
}
