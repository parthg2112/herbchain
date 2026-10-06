package types

import (
	errorsmod "cosmossdk.io/errors"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
		PortId: PortID,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	// The IBC router binds this module under its name, so the port is fixed.
	if gs.PortId != PortID {
		return errorsmod.Wrapf(ErrInvalidGenesis, "port id must be %q, got %q", PortID, gs.PortId)
	}

	seen := make(map[[2]string]struct{}, len(gs.Attestations))
	for i, a := range gs.Attestations {
		if err := a.Validate(); err != nil {
			return errorsmod.Wrapf(err, "attestations[%d]", i)
		}
		key := [2]string{a.ChannelId, a.BatchId}
		if _, dup := seen[key]; dup {
			return errorsmod.Wrapf(ErrInvalidGenesis, "duplicate attestation %s/%s", a.ChannelId, a.BatchId)
		}
		seen[key] = struct{}{}
	}

	seen = make(map[[2]string]struct{}, len(gs.SentAttestations))
	for i, s := range gs.SentAttestations {
		if err := s.Validate(); err != nil {
			return errorsmod.Wrapf(err, "sent_attestations[%d]", i)
		}
		key := [2]string{s.ChannelId, s.BatchId}
		if _, dup := seen[key]; dup {
			return errorsmod.Wrapf(ErrInvalidGenesis, "duplicate sent attestation %s/%s", s.ChannelId, s.BatchId)
		}
		seen[key] = struct{}{}
	}

	return gs.Params.Validate()
}

// PacketData returns the packet payload the attestation was built from.
func (a Attestation) PacketData() AttestationPacketData {
	return AttestationPacketData{
		BatchId:       a.BatchId,
		HerbType:      a.HerbType,
		Farmer:        a.Farmer,
		Status:        a.Status,
		LabReportHash: a.LabReportHash,
		SourceHeight:  a.SourceHeight,
	}
}

// Validate checks a stored attestation.
func (a Attestation) Validate() error {
	if err := host.ChannelIdentifierValidator(a.ChannelId); err != nil {
		return errorsmod.Wrapf(ErrInvalidGenesis, "channel id: %s", err)
	}
	if err := a.PacketData().ValidateBasic(); err != nil {
		return err
	}
	if a.Sequence == 0 {
		return errorsmod.Wrap(ErrInvalidGenesis, "sequence must be positive")
	}
	return nil
}

// Validate checks a stored sent-attestation record.
func (s SentAttestation) Validate() error {
	if err := host.ChannelIdentifierValidator(s.ChannelId); err != nil {
		return errorsmod.Wrapf(ErrInvalidGenesis, "channel id: %s", err)
	}
	if err := ValidateBatchID(s.BatchId); err != nil {
		return err
	}
	if s.Sequence == 0 {
		return errorsmod.Wrap(ErrInvalidGenesis, "sequence must be positive")
	}
	if s.AckedSequence > s.Sequence {
		return errorsmod.Wrapf(ErrInvalidGenesis, "acked sequence %d exceeds sequence %d", s.AckedSequence, s.Sequence)
	}
	if _, ok := SentStatus_name[int32(s.Status)]; !ok || s.Status == SentStatusUnspecified {
		return errorsmod.Wrapf(ErrInvalidGenesis, "invalid status %d", s.Status)
	}
	return nil
}
