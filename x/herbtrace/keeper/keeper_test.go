package keeper_test

import (
	"strings"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/keeper"
	module "github.com/parthg2112/herbchain/x/herbtrace/module"
	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

const (
	testHeight = int64(10)
	reportHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
)

var testBlockTime = time.Unix(1_700_000_000, 0).UTC()

type fixture struct {
	ctx          sdk.Context
	keeper       keeper.Keeper
	msgServer    types.MsgServer
	queryServer  types.QueryServer
	addressCodec address.Codec
	authority    string

	farmer    string // alice
	processor string // bob
	certifier string // carol
	stranger  string // dave
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx.
		WithBlockHeight(testHeight).
		WithBlockTime(testBlockTime)

	authority := authtypes.NewModuleAddress(types.GovModuleName)
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority)

	f := &fixture{
		ctx:          ctx,
		keeper:       k,
		msgServer:    keeper.NewMsgServerImpl(k),
		queryServer:  keeper.NewQueryServerImpl(k),
		addressCodec: addressCodec,
	}
	f.authority = f.addr(t, authority)
	f.farmer = f.addr(t, seedAddr("alice"))
	f.processor = f.addr(t, seedAddr("bob"))
	f.certifier = f.addr(t, seedAddr("carol"))
	f.stranger = f.addr(t, seedAddr("dave"))

	require.NoError(t, k.Params.Set(ctx, types.NewParams([]string{f.certifier}, types.DefaultMaxCertifications)))
	return f
}

func seedAddr(seed string) sdk.AccAddress {
	return sdk.AccAddress(seed + strings.Repeat("_", 20-len(seed)))
}

func (f *fixture) addr(t *testing.T, bz []byte) string {
	t.Helper()
	s, err := f.addressCodec.BytesToString(bz)
	require.NoError(t, err)
	return s
}

func (f *fixture) createMsg(id string) *types.MsgCreateBatch {
	return &types.MsgCreateBatch{
		Farmer:         f.farmer,
		Id:             id,
		HerbType:       "ashwagandha",
		OriginLatMicro: 26_912_400,
		OriginLngMicro: -75_787_300,
		HarvestTime:    testBlockTime.Unix() - 3600,
		QuantityGrams:  25_000,
		QualityGrade:   "A",
		Certifications: []string{"organic", "ayush"},
		Location:       "Jaipur farm",
	}
}

// createAt creates a batch owned by the farmer and drives it to target via the legal path.
func (f *fixture) createAt(t *testing.T, id string, target types.BatchStatus) {
	t.Helper()
	_, err := f.msgServer.CreateBatch(f.ctx, f.createMsg(id))
	require.NoError(t, err)
	for _, s := range pathTo(target) {
		f.step(t, id, s)
	}
	b, err := f.keeper.GetBatch(f.ctx, id)
	require.NoError(t, err)
	require.Equal(t, target, b.Status)
}

func pathTo(target types.BatchStatus) []types.BatchStatus {
	if target == types.StatusRejected {
		return []types.BatchStatus{types.StatusProcessed, types.StatusRejected}
	}
	full := []types.BatchStatus{
		types.StatusProcessed, types.StatusTested, types.StatusPackaged, types.StatusShipped, types.StatusDelivered,
	}
	for i, s := range full {
		if s == target {
			return full[:i+1]
		}
	}
	return nil
}

func (f *fixture) step(t *testing.T, id string, s types.BatchStatus) {
	t.Helper()
	if types.IsLabStatus(s) {
		_, err := f.msgServer.RecordLabResult(f.ctx, &types.MsgRecordLabResult{
			Certifier: f.certifier, BatchId: id, Passed: s == types.StatusTested, ReportHash: reportHash,
		})
		require.NoError(t, err)
		return
	}
	b, err := f.keeper.GetBatch(f.ctx, id)
	require.NoError(t, err)
	_, err = f.msgServer.UpdateStatus(f.ctx, &types.MsgUpdateStatus{Owner: b.Owner, BatchId: id, Status: s})
	require.NoError(t, err)
}

// freshEvents resets the event manager so a test can assert on events of the next call only.
func (f *fixture) freshEvents() {
	f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())
}

func requireTypedEvent(t *testing.T, ctx sdk.Context, want proto.Message) {
	t.Helper()
	name := proto.MessageName(want)
	for _, ev := range ctx.EventManager().ABCIEvents() {
		if ev.Type != name {
			continue
		}
		got, err := sdk.ParseTypedEvent(ev)
		require.NoError(t, err)
		require.Equal(t, want, got)
		return
	}
	require.Failf(t, "event not emitted", "%s", name)
}

func (f *fixture) history(t *testing.T, id string) []types.HistoryEntry {
	t.Helper()
	res, err := f.queryServer.BatchHistory(f.ctx, &types.QueryBatchHistoryRequest{Id: id})
	require.NoError(t, err)
	return res.Entries
}

func (f *fixture) idsByOwner(t *testing.T, owner string) []string {
	t.Helper()
	res, err := f.queryServer.BatchesByOwner(f.ctx, &types.QueryBatchesByOwnerRequest{Owner: owner})
	require.NoError(t, err)
	return batchIDs(res.Batches)
}

func (f *fixture) idsByStatus(t *testing.T, s types.BatchStatus) []string {
	t.Helper()
	res, err := f.queryServer.BatchesByStatus(f.ctx, &types.QueryBatchesByStatusRequest{Status: s})
	require.NoError(t, err)
	return batchIDs(res.Batches)
}

func batchIDs(bs []types.Batch) []string {
	ids := make([]string, 0, len(bs))
	for _, b := range bs {
		ids = append(ids, b.Id)
	}
	return ids
}
