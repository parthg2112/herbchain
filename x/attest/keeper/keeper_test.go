package keeper_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/attest/keeper"
	module "github.com/parthg2112/herbchain/x/attest/module"
	"github.com/parthg2112/herbchain/x/attest/types"
	herbtracetypes "github.com/parthg2112/herbchain/x/herbtrace/types"
)

const (
	testHeight  = 42
	testChannel = "channel-0"
	testHash    = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
)

var testTime = time.Unix(1_700_000_000, 0).UTC()

type fixture struct {
	ctx          sdk.Context
	addressCodec address.Codec
	keeper       keeper.Keeper
	herbtrace    *mockHerbtrace
	sender       *mockSender
	creator      string
	farmer       string
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx.
		WithBlockHeight(testHeight).
		WithBlockTime(testTime)

	herbtrace := &mockHerbtrace{batches: map[string]herbtracetypes.Batch{}}
	sender := &mockSender{}
	k := keeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		encCfg.Codec,
		addressCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		nil,
		herbtrace,
	).WithPacketSender(sender)

	require.NoError(t, k.InitGenesis(ctx, *types.DefaultGenesis()))

	creator, err := addressCodec.BytesToString([]byte("creator_____________"))
	require.NoError(t, err)
	farmer, err := addressCodec.BytesToString([]byte("farmer______________"))
	require.NoError(t, err)

	return &fixture{ctx: ctx, addressCodec: addressCodec, keeper: k, herbtrace: herbtrace, sender: sender, creator: creator, farmer: farmer}
}

// addBatch stores a valid batch in the mock herbtrace keeper.
func (f *fixture) addBatch(id string, status herbtracetypes.BatchStatus) herbtracetypes.Batch {
	b := herbtracetypes.Batch{
		Id:            id,
		HerbType:      "ashwagandha",
		Farmer:        f.farmer,
		Owner:         f.farmer,
		Status:        status,
		LabReportHash: testHash,
	}
	f.herbtrace.batches[id] = b
	return b
}

func (f *fixture) timeout() uint64 {
	return uint64(testTime.Add(time.Minute).UnixNano())
}

type mockHerbtrace struct {
	batches map[string]herbtracetypes.Batch
}

func (m *mockHerbtrace) GetBatch(_ context.Context, id string) (herbtracetypes.Batch, error) {
	b, ok := m.batches[id]
	if !ok {
		return herbtracetypes.Batch{}, errorsmod.Wrapf(herbtracetypes.ErrBatchNotFound, "id %s", id)
	}
	return b, nil
}

type sentPacket struct {
	port, channel string
	timeoutHeight clienttypes.Height
	timeout       uint64
	data          []byte
}

type mockSender struct {
	seq     uint64
	err     error
	packets []sentPacket
}

func (m *mockSender) SendPacket(_ sdk.Context, port, channel string, th clienttypes.Height, ts uint64, data []byte) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.seq++
	m.packets = append(m.packets, sentPacket{port: port, channel: channel, timeoutHeight: th, timeout: ts, data: data})
	return m.seq, nil
}

var errChannelNotFound = errors.New("channel not found")
