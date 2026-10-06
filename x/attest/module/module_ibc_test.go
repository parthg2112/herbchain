package attest_test

import (
	"testing"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/attest/keeper"
	attest "github.com/parthg2112/herbchain/x/attest/module"
	"github.com/parthg2112/herbchain/x/attest/types"
)

const testHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

func setup(t *testing.T) (sdk.Context, keeper.Keeper, attest.IBCModule) {
	t.Helper()
	encCfg := moduletestutil.MakeTestEncodingConfig(attest.AppModule{})
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("t")).Ctx.WithBlockHeight(10)
	k := keeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		encCfg.Codec,
		addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		nil,
		nil,
	)
	return ctx, k, attest.NewIBCModule(encCfg.Codec, k)
}

func packetBytes(t *testing.T) []byte {
	t.Helper()
	bz, err := types.AttestationPacketData{
		BatchId:       "BATCH-1",
		HerbType:      "tulsi",
		Farmer:        "herb1trjvk0a7m5ta4ms2xayet88d9pe99dmynz25kw",
		Status:        types.StatusTested,
		LabReportHash: testHash,
		SourceHeight:  3,
	}.GetBytes()
	require.NoError(t, err)
	return bz
}

func TestChannelHandshake(t *testing.T) {
	ctx, _, im := setup(t)
	cp := channeltypes.NewCounterparty(types.PortID, "channel-1")

	v, err := im.OnChanOpenInit(ctx, channeltypes.UNORDERED, nil, types.PortID, "channel-0", cp, "")
	require.NoError(t, err)
	require.Equal(t, types.Version, v)
	_, err = im.OnChanOpenInit(ctx, channeltypes.UNORDERED, nil, types.PortID, "channel-0", cp, types.Version)
	require.NoError(t, err)
	_, err = im.OnChanOpenInit(ctx, channeltypes.UNORDERED, nil, types.PortID, "channel-0", cp, "ics20-1")
	require.ErrorIs(t, err, types.ErrInvalidVersion)
	_, err = im.OnChanOpenInit(ctx, channeltypes.ORDERED, nil, types.PortID, "channel-0", cp, types.Version)
	require.ErrorIs(t, err, types.ErrInvalidChannelOrder)

	v, err = im.OnChanOpenTry(ctx, channeltypes.UNORDERED, nil, types.PortID, "channel-0", cp, types.Version)
	require.NoError(t, err)
	require.Equal(t, types.Version, v)
	_, err = im.OnChanOpenTry(ctx, channeltypes.UNORDERED, nil, types.PortID, "channel-0", cp, "ics20-1")
	require.ErrorIs(t, err, types.ErrInvalidVersion)
	_, err = im.OnChanOpenTry(ctx, channeltypes.ORDERED, nil, types.PortID, "channel-0", cp, types.Version)
	require.ErrorIs(t, err, types.ErrInvalidChannelOrder)

	require.NoError(t, im.OnChanOpenAck(ctx, types.PortID, "channel-0", "channel-1", types.Version))
	require.ErrorIs(t, im.OnChanOpenAck(ctx, types.PortID, "channel-0", "channel-1", "x"), types.ErrInvalidVersion)
	require.NoError(t, im.OnChanOpenConfirm(ctx, types.PortID, "channel-0"))
	require.Error(t, im.OnChanCloseInit(ctx, types.PortID, "channel-0"))
	require.NoError(t, im.OnChanCloseConfirm(ctx, types.PortID, "channel-0"))
}

func TestOnRecvPacket(t *testing.T) {
	ctx, k, im := setup(t)
	packet := channeltypes.Packet{Sequence: 1, SourcePort: types.PortID, SourceChannel: "channel-5",
		DestinationPort: types.PortID, DestinationChannel: "channel-0", Data: packetBytes(t)}

	ack := im.OnRecvPacket(ctx, types.Version, packet, nil)
	require.True(t, ack.Success())
	var res channeltypes.Acknowledgement
	require.NoError(t, channeltypes.SubModuleCdc.UnmarshalJSON(ack.Acknowledgement(), &res))
	require.JSONEq(t, `{"received":true}`, string(res.GetResult()))

	has, err := k.Attestations.Has(ctx, collections.Join("channel-0", "BATCH-1"))
	require.NoError(t, err)
	require.True(t, has)

	dup := im.OnRecvPacket(ctx, types.Version, packet, nil)
	require.False(t, dup.Success())
	require.Contains(t, string(dup.Acknowledgement()), "attest/1102")
	require.Contains(t, ctx.EventManager().Events()[len(ctx.EventManager().Events())-1].Type, "EventAttestationRejected")

	packet.Data = []byte("garbage")
	bad := im.OnRecvPacket(ctx, types.Version, packet, nil)
	require.False(t, bad.Success())
	require.Contains(t, string(bad.Acknowledgement()), "attest/1101")

	empty, err := (&types.AttestPacketData{}).Marshal()
	require.NoError(t, err)
	packet.Data = empty
	require.False(t, im.OnRecvPacket(ctx, types.Version, packet, nil).Success())
}

func TestOnAcknowledgementAndTimeout(t *testing.T) {
	ctx, k, im := setup(t)
	key := collections.Join("channel-0", "BATCH-1")
	packet := channeltypes.Packet{Sequence: 4, SourcePort: types.PortID, SourceChannel: "channel-0", Data: packetBytes(t)}
	pending := types.SentAttestation{ChannelId: "channel-0", BatchId: "BATCH-1", Sequence: 4, Status: types.SentStatusPending}

	require.NoError(t, k.SentAttestations.Set(ctx, key, pending))
	errAck := channeltypes.NewErrorAcknowledgementWithCodespace(types.ErrDuplicateAttestation)
	require.NoError(t, im.OnAcknowledgementPacket(ctx, types.Version, packet, errAck.Acknowledgement(), nil))
	got, err := k.SentAttestations.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, types.SentStatusErrored, got.Status)
	require.Contains(t, got.Error, "attest/1102")

	require.NoError(t, k.SentAttestations.Set(ctx, key, pending))
	require.NoError(t, im.OnTimeoutPacket(ctx, types.Version, packet, nil))
	got, err = k.SentAttestations.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, types.SentStatusTimedOut, got.Status)

	require.ErrorIs(t, im.OnAcknowledgementPacket(ctx, types.Version, packet, []byte("nope"), nil), types.ErrInvalidAck)
	packet.Data = []byte("garbage")
	okAck := channeltypes.NewResultAcknowledgement([]byte(`{"received":true}`))
	require.ErrorIs(t, im.OnAcknowledgementPacket(ctx, types.Version, packet, okAck.Acknowledgement(), nil), types.ErrInvalidPacket)
	require.ErrorIs(t, im.OnTimeoutPacket(ctx, types.Version, packet, nil), types.ErrInvalidPacket)
}
