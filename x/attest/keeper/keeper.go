package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	"github.com/parthg2112/herbchain/x/attest/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	Port collections.Item[string]
	// Attestations holds received attestations keyed by (local channel, batch id).
	Attestations collections.Map[collections.Pair[string, string], types.Attestation]
	// SentAttestations tracks the latest send per (source channel, batch id).
	SentAttestations collections.Map[collections.Pair[string, string], types.SentAttestation]

	ibcKeeperFn func() *ibckeeper.Keeper
	// packetSender overrides the IBC channel keeper; set only in tests.
	packetSender types.PacketSender

	herbtraceKeeper types.HerbtraceKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	ibcKeeperFn func() *ibckeeper.Keeper,
	herbtraceKeeper types.HerbtraceKeeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)
	pairKey := collections.PairKeyCodec(collections.StringKey, collections.StringKey)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,

		herbtraceKeeper: herbtraceKeeper,
		ibcKeeperFn:     ibcKeeperFn,
		Port:            collections.NewItem(sb, types.PortKey, "port", collections.StringValue),
		Params:          collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Attestations: collections.NewMap(sb, types.AttestationsKey, "attestations", pairKey,
			codec.CollValue[types.Attestation](cdc)),
		SentAttestations: collections.NewMap(sb, types.SentAttestationsKey, "sent_attestations", pairKey,
			codec.CollValue[types.SentAttestation](cdc)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

func (k Keeper) sender() (types.PacketSender, error) {
	if k.packetSender != nil {
		return k.packetSender, nil
	}
	if k.ibcKeeperFn == nil {
		return nil, types.ErrIBCUnavailable
	}
	ibcKeeper := k.ibcKeeperFn()
	if ibcKeeper == nil || ibcKeeper.ChannelKeeper == nil {
		return nil, types.ErrIBCUnavailable
	}
	return ibcKeeper.ChannelKeeper, nil
}
