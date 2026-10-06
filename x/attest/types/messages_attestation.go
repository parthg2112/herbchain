package types

// NewMsgSendAttestation builds a MsgSendAttestation; timeoutTimestamp is absolute unix nanoseconds.
func NewMsgSendAttestation(creator, channelID, batchID string, timeoutTimestamp uint64) *MsgSendAttestation {
	return &MsgSendAttestation{
		Creator:          creator,
		BatchId:          batchID,
		ChannelId:        channelID,
		TimeoutTimestamp: timeoutTimestamp,
	}
}
