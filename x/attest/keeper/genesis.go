package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"

	"github.com/parthg2112/herbchain/x/attest/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := k.Port.Set(ctx, genState.PortId); err != nil {
		return errorsmod.Wrap(err, "set port")
	}
	if err := k.Params.Set(ctx, genState.Params); err != nil {
		return errorsmod.Wrap(err, "set params")
	}
	for _, a := range genState.Attestations {
		if err := k.Attestations.Set(ctx, collections.Join(a.ChannelId, a.BatchId), a); err != nil {
			return errorsmod.Wrapf(err, "set attestation %s/%s", a.ChannelId, a.BatchId)
		}
	}
	for _, s := range genState.SentAttestations {
		if err := k.SentAttestations.Set(ctx, collections.Join(s.ChannelId, s.BatchId), s); err != nil {
			return errorsmod.Wrapf(err, "set sent attestation %s/%s", s.ChannelId, s.BatchId)
		}
	}
	return nil
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	var err error

	genesis := types.DefaultGenesis()
	genesis.Params, err = k.Params.Get(ctx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "get params")
	}
	genesis.PortId, err = k.Port.Get(ctx)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return nil, errorsmod.Wrap(err, "get port")
	}

	err = k.Attestations.Walk(ctx, nil, func(_ collections.Pair[string, string], a types.Attestation) (bool, error) {
		genesis.Attestations = append(genesis.Attestations, a)
		return false, nil
	})
	if err != nil {
		return nil, errorsmod.Wrap(err, "export attestations")
	}

	err = k.SentAttestations.Walk(ctx, nil, func(_ collections.Pair[string, string], s types.SentAttestation) (bool, error) {
		genesis.SentAttestations = append(genesis.SentAttestations, s)
		return false, nil
	})
	if err != nil {
		return nil, errorsmod.Wrap(err, "export sent attestations")
	}

	return genesis, nil
}
