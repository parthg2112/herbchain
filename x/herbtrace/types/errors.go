package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/herbtrace module sentinel errors
var (
	ErrInvalidSigner     = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrBatchNotFound     = errors.Register(ModuleName, 1101, "batch not found")
	ErrBatchExists       = errors.Register(ModuleName, 1102, "batch already exists")
	ErrInvalidBatch      = errors.Register(ModuleName, 1103, "invalid batch field")
	ErrInvalidTransition = errors.Register(ModuleName, 1104, "invalid status transition")
	ErrUnauthorized      = errors.Register(ModuleName, 1105, "signer not authorized for this action")
	ErrTerminalStatus    = errors.Register(ModuleName, 1106, "batch is in a terminal status")
	ErrInvalidReportHash = errors.Register(ModuleName, 1107, "invalid lab report hash")
	ErrInvalidParams     = errors.Register(ModuleName, 1108, "invalid params")
	ErrInvalidGenesis    = errors.Register(ModuleName, 1109, "invalid genesis state")
	ErrInvalidAddress    = errors.Register(ModuleName, 1110, "invalid address")
)
