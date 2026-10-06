package cli

import (
	"time"

	"github.com/spf13/cast"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/parthg2112/herbchain/x/attest/types"
)

// CmdSendAttestation() returns the Attestation send packet command.
// This command does not use AutoCLI because it gives a better UX to do not.
func CmdSendAttestation() *cobra.Command {
	flagPacketTimeoutTimestamp := "packet-timeout-timestamp"

	cmd := &cobra.Command{
		Use:   "send-attestation [src-port] [src-channel] [batch-id] [herb-type] [farmer] [status] [lab-report-hash] [source-height]",
		Short: "Send a attestation over IBC",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			creator := clientCtx.GetFromAddress().String()
			srcPort := args[0]
			srcChannel := args[1]

			argBatchId := args[2]
			argHerbType := args[3]
			argFarmer := args[4]
			argStatus := args[5]
			argLabReportHash := args[6]
			argSourceHeight, err := cast.ToInt64E(args[7])
			if err != nil {
				return err
			}

			// Get the relative timeout timestamp
			timeoutTimestamp, err := cmd.Flags().GetUint64(flagPacketTimeoutTimestamp)
			if err != nil {
				return err
			}

			if timeoutTimestamp != 0 {
				now := time.Now().UnixNano()
				if now <= 0 {
					return err
				}
				timeoutTimestamp = uint64(now) + timeoutTimestamp
			}

			msg := types.NewMsgSendAttestation(creator, srcPort, srcChannel, timeoutTimestamp, argBatchId, argHerbType, argFarmer, argStatus, argLabReportHash, argSourceHeight)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64(flagPacketTimeoutTimestamp, DefaultRelativePacketTimeoutTimestamp, "Packet timeout timestamp in nanoseconds. Default is 10 minutes.")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
