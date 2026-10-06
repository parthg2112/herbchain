package attest

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/parthg2112/herbchain/x/attest/types"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Shows the parameters of the module",
				},
				{
					RpcMethod:      "Attestation",
					Use:            "attestation [channel-id] [batch-id]",
					Short:          "Show an attestation received on a local channel",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "channel_id"}, {ProtoField: "batch_id"}},
				},
				{
					RpcMethod: "Attestations",
					Use:       "attestations",
					Short:     "List received attestations",
				},
				{
					RpcMethod:      "SentAttestation",
					Use:            "sent-attestation [channel-id] [batch-id]",
					Short:          "Show the delivery status of an attestation sent on a channel",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "channel_id"}, {ProtoField: "batch_id"}},
				},
				{
					RpcMethod: "SentAttestations",
					Use:       "sent-attestations",
					Short:     "List sent attestations and their delivery status",
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // skipped because authority gated
				},
				{
					RpcMethod: "SendAttestation",
					Skip:      true, // custom command converts a relative timeout to absolute
				},
			},
		},
	}
}
