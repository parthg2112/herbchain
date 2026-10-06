package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/attest module sentinel errors
var (
	ErrInvalidSigner        = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInvalidPacket        = errors.Register(ModuleName, 1101, "invalid attestation packet")
	ErrDuplicateAttestation = errors.Register(ModuleName, 1102, "attestation already received")
	ErrBatchNotAttestable   = errors.Register(ModuleName, 1103, "batch has not passed lab testing")
	ErrAttestationPending   = errors.Register(ModuleName, 1104, "attestation already in flight")
	ErrInvalidAck           = errors.Register(ModuleName, 1105, "invalid acknowledgement")
	ErrInvalidGenesis       = errors.Register(ModuleName, 1106, "invalid genesis state")
	ErrInvalidRequest       = errors.Register(ModuleName, 1107, "invalid request")
	ErrIBCUnavailable       = errors.Register(ModuleName, 1108, "ibc keeper unavailable")
	ErrInvalidPacketTimeout = errors.Register(ModuleName, 1500, "invalid packet timeout")
	ErrInvalidVersion       = errors.Register(ModuleName, 1501, "invalid version")
	ErrInvalidChannelOrder  = errors.Register(ModuleName, 1502, "invalid channel ordering")
)
