package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// authority may execute MsgUpdateParams; normally the x/gov module account.
	authority []byte

	Schema     collections.Schema
	Params     collections.Item[types.Params]
	Batches    collections.Map[string, types.Batch]
	History    collections.Map[collections.Pair[string, uint64], types.HistoryEntry]
	HistorySeq collections.Map[string, uint64]
	// OwnerIndex and StatusIndex are derived from Batches; only setBatch writes them.
	OwnerIndex  collections.KeySet[collections.Pair[sdk.AccAddress, string]]
	StatusIndex collections.KeySet[collections.Pair[int32, string]]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,

		Params:  collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Batches: collections.NewMap(sb, types.BatchesKey, "batches", collections.StringKey, codec.CollValue[types.Batch](cdc)),
		History: collections.NewMap(sb, types.HistoryKey, "history",
			collections.PairKeyCodec(collections.StringKey, collections.Uint64Key), codec.CollValue[types.HistoryEntry](cdc)),
		HistorySeq: collections.NewMap(sb, types.HistorySeqKey, "history_seq", collections.StringKey, collections.Uint64Value),
		OwnerIndex: collections.NewKeySet(sb, types.OwnerIndexKey, "owner_index",
			collections.PairKeyCodec(sdk.AccAddressKey, collections.StringKey)),
		StatusIndex: collections.NewKeySet(sb, types.StatusIndexKey, "status_index",
			collections.PairKeyCodec(collections.Int32Key, collections.StringKey)),
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
