package cli

import (
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/parthg2112/herbchain/x/attest/types"
)

const (
	flagTimeout          = "timeout"
	flagTimeoutTimestamp = "timeout-timestamp"
	defaultTimeout       = 10 * time.Minute
)

// CmdSendAttestation returns the send-attestation command. It is hand-written because the
// relative --timeout must be converted to an absolute timestamp on the client.
func CmdSendAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-attestation [channel-id] [batch-id]",
		Short: "Attest a lab-tested batch to the chain behind an attest channel",
		Long: `Reads the batch from x/herbtrace and sends it as an IBC attestation packet.
The batch must be TESTED, PACKAGED, SHIPPED or DELIVERED.`,
		Example: fmt.Sprintf("%s tx %s send-attestation channel-0 BATCH-001 --timeout 5m --from alice",
			"herbchaind", types.ModuleName),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			timeoutTimestamp, err := cmd.Flags().GetUint64(flagTimeoutTimestamp)
			if err != nil {
				return err
			}
			if timeoutTimestamp == 0 {
				relative, err := cmd.Flags().GetDuration(flagTimeout)
				if err != nil {
					return err
				}
				if relative <= 0 {
					return fmt.Errorf("--%s must be positive, got %s", flagTimeout, relative)
				}
				timeoutTimestamp = uint64(time.Now().Add(relative).UnixNano())
			}

			msg := types.NewMsgSendAttestation(clientCtx.GetFromAddress().String(), args[0], args[1], timeoutTimestamp)
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Duration(flagTimeout, defaultTimeout, "Packet timeout relative to local wall-clock time")
	cmd.Flags().Uint64(flagTimeoutTimestamp, 0, "Absolute packet timeout in unix nanoseconds; overrides --timeout")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
