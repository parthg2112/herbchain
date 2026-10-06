package attest

import (
	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	"github.com/parthg2112/herbchain/x/attest/keeper"
	"github.com/parthg2112/herbchain/x/attest/types"
)

// IBCModule implements the ICS26 callbacks for the attest port.
type IBCModule struct {
	cdc    codec.Codec
	keeper keeper.Keeper
}

// NewIBCModule creates a new IBCModule given the associated keeper
func NewIBCModule(cdc codec.Codec, k keeper.Keeper) IBCModule {
	return IBCModule{
		cdc:    cdc,
		keeper: k,
	}
}

// Attestations are independent, and on ORDERED channels a timeout would close the channel.
func validateOrder(order channeltypes.Order) error {
	if order != channeltypes.UNORDERED {
		return errorsmod.Wrapf(types.ErrInvalidChannelOrder, "expected %s, got %s", channeltypes.UNORDERED, order)
	}
	return nil
}

// OnChanOpenInit implements the IBCModule interface
func (im IBCModule) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	counterparty channeltypes.Counterparty,
	version string,
) (string, error) {
	if err := validateOrder(order); err != nil {
		return "", err
	}
	if version == "" {
		return types.Version, nil
	}
	if version != types.Version {
		return "", errorsmod.Wrapf(types.ErrInvalidVersion, "got %s, expected %s", version, types.Version)
	}
	return version, nil
}

// OnChanOpenTry implements the IBCModule interface
func (im IBCModule) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID,
	channelID string,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	if err := validateOrder(order); err != nil {
		return "", err
	}
	if counterpartyVersion != types.Version {
		return "", errorsmod.Wrapf(types.ErrInvalidVersion, "invalid counterparty version: got %s, expected %s", counterpartyVersion, types.Version)
	}
	return types.Version, nil
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCModule) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID,
	counterpartyChannelID,
	counterpartyVersion string,
) error {
	if counterpartyVersion != types.Version {
		return errorsmod.Wrapf(types.ErrInvalidVersion, "invalid counterparty version: got %s, expected %s", counterpartyVersion, types.Version)
	}
	return nil
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCModule) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	return nil
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCModule) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "user cannot close channel")
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCModule) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	return nil
}

// OnRecvPacket implements the IBCModule interface. Any error becomes an error ack, which
// makes core IBC discard this callback's state writes.
func (im IBCModule) OnRecvPacket(
	ctx sdk.Context,
	channelVersion string,
	modulePacket channeltypes.Packet,
	relayer sdk.AccAddress,
) ibcexported.Acknowledgement {
	data, err := decodeAttestation(modulePacket.GetData())
	if err != nil {
		return im.rejectPacket(ctx, modulePacket, "", err)
	}

	packetAck, err := im.keeper.OnRecvAttestationPacket(ctx, modulePacket, data)
	if err != nil {
		return im.rejectPacket(ctx, modulePacket, data.BatchId, err)
	}

	bz, err := im.cdc.MarshalJSON(&packetAck)
	if err != nil {
		return im.rejectPacket(ctx, modulePacket, data.BatchId, errorsmod.Wrap(sdkerrors.ErrJSONMarshal, err.Error()))
	}
	return channeltypes.NewResultAcknowledgement(bz)
}

// rejectPacket emits the reason (core IBC re-emits it as an error event) and returns an error ack
// carrying only the deterministic codespace/code.
func (im IBCModule) rejectPacket(ctx sdk.Context, packet channeltypes.Packet, batchID string, err error) ibcexported.Acknowledgement {
	_ = ctx.EventManager().EmitTypedEvent(&types.EventAttestationRejected{
		ChannelId: packet.DestinationChannel,
		BatchId:   batchID,
		Sequence:  packet.Sequence,
		Reason:    err.Error(),
	})
	return channeltypes.NewErrorAcknowledgementWithCodespace(err)
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im IBCModule) OnAcknowledgementPacket(
	ctx sdk.Context,
	channelVersion string,
	modulePacket channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	var ack channeltypes.Acknowledgement
	if err := channeltypes.SubModuleCdc.UnmarshalJSON(acknowledgement, &ack); err != nil {
		return errorsmod.Wrapf(types.ErrInvalidAck, "cannot unmarshal packet acknowledgement: %s", err)
	}

	data, err := decodeAttestation(modulePacket.GetData())
	if err != nil {
		return err
	}
	return im.keeper.OnAcknowledgementAttestationPacket(ctx, modulePacket, data, ack)
}

// OnTimeoutPacket implements the IBCModule interface
func (im IBCModule) OnTimeoutPacket(
	ctx sdk.Context,
	channelVersion string,
	modulePacket channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	data, err := decodeAttestation(modulePacket.GetData())
	if err != nil {
		return err
	}
	return im.keeper.OnTimeoutAttestationPacket(ctx, modulePacket, data)
}

func decodeAttestation(bz []byte) (types.AttestationPacketData, error) {
	var envelope types.AttestPacketData
	if err := envelope.Unmarshal(bz); err != nil {
		return types.AttestationPacketData{}, errorsmod.Wrapf(types.ErrInvalidPacket, "cannot unmarshal packet data: %s", err)
	}

	packet, ok := envelope.Packet.(*types.AttestPacketData_AttestationPacket)
	if !ok || packet.AttestationPacket == nil {
		return types.AttestationPacketData{}, errorsmod.Wrapf(types.ErrInvalidPacket, "unrecognized %s packet type: %T", types.ModuleName, envelope.Packet)
	}
	return *packet.AttestationPacket, nil
}
