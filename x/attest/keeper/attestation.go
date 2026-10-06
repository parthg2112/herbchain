package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"

	"github.com/parthg2112/herbchain/x/attest/types"
	herbtracetypes "github.com/parthg2112/herbchain/x/herbtrace/types"
)

// wireStatus maps herbtrace statuses reachable only after a passed lab test to the packet vocabulary.
func wireStatus(s herbtracetypes.BatchStatus) (string, bool) {
	switch s {
	case herbtracetypes.StatusTested:
		return types.StatusTested, true
	case herbtracetypes.StatusPackaged:
		return types.StatusPackaged, true
	case herbtracetypes.StatusShipped:
		return types.StatusShipped, true
	case herbtracetypes.StatusDelivered:
		return types.StatusDelivered, true
	default:
		return "", false
	}
}

// SendAttestation builds an attestation packet from x/herbtrace state, sends it on channelID
// and records it as PENDING. A resend is allowed once the previous packet has settled (see docs/design.md).
func (k Keeper) SendAttestation(ctx context.Context, sender, channelID, batchID string, timeoutTimestamp uint64) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if err := host.ChannelIdentifierValidator(channelID); err != nil {
		return 0, errorsmod.Wrapf(types.ErrInvalidRequest, "channel id: %s", err)
	}
	if err := types.ValidateBatchID(batchID); err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidRequest, err.Error())
	}
	if blockNanos := sdkCtx.BlockTime().UnixNano(); blockNanos < 0 || timeoutTimestamp <= uint64(blockNanos) {
		return 0, errorsmod.Wrapf(types.ErrInvalidPacketTimeout, "timeout %d is not after block time %d", timeoutTimestamp, blockNanos)
	}

	key := collections.Join(channelID, batchID)
	prev, err := k.SentAttestations.Get(ctx, key)
	switch {
	case err == nil && prev.Status == types.SentStatusPending:
		return 0, errorsmod.Wrapf(types.ErrAttestationPending, "%s/%s sequence %d", channelID, batchID, prev.Sequence)
	case err != nil && !errors.Is(err, collections.ErrNotFound):
		return 0, errorsmod.Wrap(err, "load sent attestation")
	}

	batch, err := k.herbtraceKeeper.GetBatch(ctx, batchID)
	if err != nil {
		return 0, errorsmod.Wrapf(err, "load batch %s", batchID)
	}
	status, ok := wireStatus(batch.Status)
	if !ok {
		return 0, errorsmod.Wrapf(types.ErrBatchNotAttestable, "batch %s is %s", batchID, batch.Status)
	}

	data := types.AttestationPacketData{
		BatchId:       batch.Id,
		HerbType:      batch.HerbType,
		Farmer:        batch.Farmer,
		Status:        status,
		LabReportHash: batch.LabReportHash,
		SourceHeight:  sdkCtx.BlockHeight(),
	}
	if err := data.ValidateBasic(); err != nil {
		return 0, errorsmod.Wrapf(err, "batch %s", batchID)
	}
	bz, err := data.GetBytes()
	if err != nil {
		return 0, errorsmod.Wrap(err, "marshal packet")
	}

	ps, err := k.sender()
	if err != nil {
		return 0, err
	}
	seq, err := ps.SendPacket(sdkCtx, types.PortID, channelID, clienttypes.ZeroHeight(), timeoutTimestamp, bz)
	if err != nil {
		return 0, errorsmod.Wrapf(err, "send packet on %s/%s", types.PortID, channelID)
	}

	sent := types.SentAttestation{
		ChannelId:     channelID,
		BatchId:       batchID,
		Sequence:      seq,
		Status:        types.SentStatusPending,
		Sender:        sender,
		SentHeight:    sdkCtx.BlockHeight(),
		UpdatedHeight: sdkCtx.BlockHeight(),
		AckedSequence: prev.AckedSequence,
	}
	if err := k.SentAttestations.Set(ctx, key, sent); err != nil {
		return 0, errorsmod.Wrap(err, "store sent attestation")
	}

	return seq, sdkCtx.EventManager().EmitTypedEvent(&types.EventAttestationSent{
		ChannelId: channelID,
		BatchId:   batchID,
		Sequence:  seq,
		Sender:    sender,
		Status:    status,
	})
}

// OnRecvAttestationPacket validates and stores a received attestation. Errors become error acks.
func (k Keeper) OnRecvAttestationPacket(ctx context.Context, packet channeltypes.Packet, data types.AttestationPacketData) (types.AttestationPacketAck, error) {
	if err := data.ValidateBasic(); err != nil {
		return types.AttestationPacketAck{}, err
	}

	// Key by the local channel: source channel ids are only unique per counterparty.
	key := collections.Join(packet.DestinationChannel, data.BatchId)
	has, err := k.Attestations.Has(ctx, key)
	if err != nil {
		return types.AttestationPacketAck{}, errorsmod.Wrap(err, "load attestation")
	}
	if has {
		return types.AttestationPacketAck{}, errorsmod.Wrapf(types.ErrDuplicateAttestation, "%s/%s", packet.DestinationChannel, data.BatchId)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	att := types.Attestation{
		ChannelId:      packet.DestinationChannel,
		SourcePort:     packet.SourcePort,
		SourceChannel:  packet.SourceChannel,
		BatchId:        data.BatchId,
		HerbType:       data.HerbType,
		Farmer:         data.Farmer,
		Status:         data.Status,
		LabReportHash:  data.LabReportHash,
		SourceHeight:   data.SourceHeight,
		Sequence:       packet.Sequence,
		ReceivedHeight: sdkCtx.BlockHeight(),
		ReceivedTime:   sdkCtx.BlockTime().Unix(),
	}
	if err := k.Attestations.Set(ctx, key, att); err != nil {
		return types.AttestationPacketAck{}, errorsmod.Wrap(err, "store attestation")
	}

	err = sdkCtx.EventManager().EmitTypedEvent(&types.EventAttestationReceived{
		ChannelId:     att.ChannelId,
		SourceChannel: att.SourceChannel,
		BatchId:       att.BatchId,
		Sequence:      att.Sequence,
	})
	return types.AttestationPacketAck{Received: true}, err
}

// OnAcknowledgementAttestationPacket records the counterparty's verdict on a sent attestation.
func (k Keeper) OnAcknowledgementAttestationPacket(ctx context.Context, packet channeltypes.Packet, data types.AttestationPacketData, ack channeltypes.Acknowledgement) error {
	switch resp := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Error:
		return k.recordOutcome(ctx, packet, data.BatchId, types.SentStatusErrored, resp.Error)
	case *channeltypes.Acknowledgement_Result:
		var packetAck types.AttestationPacketAck
		// A malformed result is recorded rather than returned, so the packet can still be cleared.
		if err := k.cdc.UnmarshalJSON(resp.Result, &packetAck); err != nil {
			return k.recordOutcome(ctx, packet, data.BatchId, types.SentStatusErrored, "malformed acknowledgement result")
		}
		if !packetAck.Received {
			return k.recordOutcome(ctx, packet, data.BatchId, types.SentStatusErrored, "counterparty did not record attestation")
		}
		return k.recordOutcome(ctx, packet, data.BatchId, types.SentStatusAcked, "")
	default:
		return errorsmod.Wrapf(types.ErrInvalidAck, "unexpected response type %T", resp)
	}
}

// OnTimeoutAttestationPacket marks a sent attestation as timed out.
func (k Keeper) OnTimeoutAttestationPacket(ctx context.Context, packet channeltypes.Packet, data types.AttestationPacketData) error {
	return k.recordOutcome(ctx, packet, data.BatchId, types.SentStatusTimedOut, "packet timed out")
}

func (k Keeper) recordOutcome(ctx context.Context, packet channeltypes.Packet, batchID string, status types.SentStatus, errMsg string) error {
	key := collections.Join(packet.SourceChannel, batchID)
	sent, err := k.SentAttestations.Get(ctx, key)
	if errors.Is(err, collections.ErrNotFound) || (err == nil && sent.Sequence != packet.Sequence) {
		// No record for this packet (e.g. replaced via genesis): nothing to update.
		return nil
	}
	if err != nil {
		return errorsmod.Wrap(err, "load sent attestation")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sent.Status = status
	sent.Error = errMsg
	sent.UpdatedHeight = sdkCtx.BlockHeight()
	if status == types.SentStatusAcked {
		sent.AckedSequence = sent.Sequence
	}
	if err := k.SentAttestations.Set(ctx, key, sent); err != nil {
		return errorsmod.Wrap(err, "store sent attestation")
	}

	return sdkCtx.EventManager().EmitTypedEvent(&types.EventAttestationOutcome{
		ChannelId: sent.ChannelId,
		BatchId:   sent.BatchId,
		Sequence:  sent.Sequence,
		Status:    status,
		Error:     errMsg,
	})
}
