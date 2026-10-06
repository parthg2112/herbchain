package types

func NewMsgSendAttestation(
	creator string,
	port string,
	channelID string,
	timeoutTimestamp uint64,
	batchId string,
	herbType string,
	farmer string,
	status string,
	labReportHash string,
	sourceHeight int64,
) *MsgSendAttestation {
	return &MsgSendAttestation{
		Creator:          creator,
		Port:             port,
		ChannelID:        channelID,
		TimeoutTimestamp: timeoutTimestamp,
		BatchId:          batchId,
		HerbType:         herbType,
		Farmer:           farmer,
		Status:           status,
		LabReportHash:    labReportHash,
		SourceHeight:     sourceHeight,
	}
}
