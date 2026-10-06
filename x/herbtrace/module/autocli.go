package herbtrace

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func positional(fields ...string) []*autocliv1.PositionalArgDescriptor {
	args := make([]*autocliv1.PositionalArgDescriptor, len(fields))
	for i, f := range fields {
		args[i] = &autocliv1.PositionalArgDescriptor{ProtoField: f}
	}
	return args
}

// withOptionalLast marks the final positional arg optional.
func withOptionalLast(args []*autocliv1.PositionalArgDescriptor) []*autocliv1.PositionalArgDescriptor {
	args[len(args)-1].Optional = true
	return args
}

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
					RpcMethod:      "Batch",
					Use:            "batch [id]",
					Short:          "Show a batch by id",
					PositionalArgs: positional("id"),
				},
				{
					RpcMethod: "Batches",
					Use:       "batches",
					Short:     "List all batches ordered by id",
				},
				{
					RpcMethod:      "BatchHistory",
					Use:            "batch-history [id]",
					Short:          "List a batch's history entries in seq order",
					PositionalArgs: positional("id"),
				},
				{
					RpcMethod:      "BatchesByOwner",
					Use:            "batches-by-owner [owner]",
					Short:          "List batches currently held by an owner",
					PositionalArgs: positional("owner"),
				},
				{
					RpcMethod:      "BatchesByStatus",
					Use:            "batches-by-status [status]",
					Short:          "List batches in a status (harvested|processed|tested|packaged|shipped|delivered|rejected)",
					PositionalArgs: positional("status"),
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: types.Msg_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // authority gated
				},
				{
					RpcMethod: "CreateBatch",
					Use:       "create-batch [id] [herb-type] [harvest-time] [quantity-grams] [quality-grade]",
					Short:     "Register a new HARVESTED batch owned by the signer",
					Long: "Register a new batch. harvest-time is unix seconds (<= block time); quality-grade is A, B or C.\n" +
						"Coordinates are microdegrees passed as flags; use the '=' form for negatives, e.g. --origin-lat-micro=-33868800.",
					Example:        "create-batch ASH-2026-001 ashwagandha 1767225600 25000 A --origin-lat-micro=26912400 --origin-lng-micro=75787300 --certifications organic,ayush --location \"Jaipur farm\" --from alice",
					PositionalArgs: positional("id", "herb_type", "harvest_time", "quantity_grams", "quality_grade"),
				},
				{
					RpcMethod:      "UpdateStatus",
					Use:            "update-status [batch-id] [status] [location]",
					Short:          "Move a batch one owner-driven step forward (processed|packaged|shipped|delivered)",
					Long:           "Move a batch one owner-driven step forward. Omitting location keeps the current one.",
					Example:        "update-status ASH-2026-001 processed \"Delhi processing unit\" --remark \"dried and powdered\" --from alice",
					PositionalArgs: withOptionalLast(positional("batch_id", "status", "location")),
				},
				{
					RpcMethod:      "RecordLabResult",
					Use:            "record-lab-result [batch-id] [passed] [report-hash]",
					Short:          "Record a certifier's lab result on a PROCESSED batch (passed: true -> TESTED, false -> REJECTED)",
					Example:        "record-lab-result ASH-2026-001 true <64-lowercase-hex-sha256> --from carol",
					PositionalArgs: positional("batch_id", "passed", "report_hash"),
				},
				{
					RpcMethod:      "TransferCustody",
					Use:            "transfer-custody [batch-id] [new-owner] [location]",
					Short:          "Hand a non-terminal batch to a new owner",
					Long:           "Hand a non-terminal batch to a new owner. Omitting location keeps the current one.",
					Example:        "transfer-custody ASH-2026-001 herb1... \"Mumbai warehouse\" --from alice",
					PositionalArgs: withOptionalLast(positional("batch_id", "new_owner", "location")),
				},
			},
		},
	}
}
