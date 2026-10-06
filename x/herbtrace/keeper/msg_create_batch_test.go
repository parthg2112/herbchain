package keeper_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func TestCreateBatch(t *testing.T) {
	f := initFixture(t)
	f.freshEvents()
	msg := f.createMsg("ASH-001")

	_, err := f.msgServer.CreateBatch(f.ctx, msg)
	require.NoError(t, err)

	got, err := f.keeper.GetBatch(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, types.Batch{
		Id:             msg.Id,
		HerbType:       msg.HerbType,
		Farmer:         f.farmer,
		Owner:          f.farmer,
		OriginLatMicro: msg.OriginLatMicro,
		OriginLngMicro: msg.OriginLngMicro,
		HarvestTime:    msg.HarvestTime,
		QuantityGrams:  msg.QuantityGrams,
		QualityGrade:   msg.QualityGrade,
		Certifications: msg.Certifications,
		Status:         types.StatusHarvested,
		Location:       msg.Location,
		CreatedHeight:  testHeight,
		UpdatedHeight:  testHeight,
	}, got)

	require.Equal(t, []types.HistoryEntry{{
		BatchId: "ASH-001", Seq: 0, Actor: f.farmer, Action: types.ActionCreated,
		FromStatus: types.StatusUnspecified, ToStatus: types.StatusHarvested,
		Location: msg.Location, Height: testHeight, Time: testBlockTime.Unix(),
	}}, f.history(t, "ASH-001"))
	next, err := f.keeper.HistorySeq.Get(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, uint64(1), next)

	require.Equal(t, []string{"ASH-001"}, f.idsByOwner(t, f.farmer))
	require.Equal(t, []string{"ASH-001"}, f.idsByStatus(t, types.StatusHarvested))
	requireTypedEvent(t, f.ctx, &types.EventBatchCreated{
		BatchId: "ASH-001", Farmer: f.farmer, HerbType: "ashwagandha", QuantityGrams: 25_000,
	})
}

func TestCreateBatchDuplicateID(t *testing.T) {
	f := initFixture(t)
	_, err := f.msgServer.CreateBatch(f.ctx, f.createMsg("ASH-001"))
	require.NoError(t, err)

	dup := f.createMsg("ASH-001")
	dup.Farmer = f.stranger
	_, err = f.msgServer.CreateBatch(f.ctx, dup)
	require.ErrorIs(t, err, types.ErrBatchExists)

	b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, f.farmer, b.Farmer, "original batch untouched")
}

func TestCreateBatchInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.MsgCreateBatch)
		err    error
	}{
		{"invalid farmer", func(m *types.MsgCreateBatch) { m.Farmer = "not-bech32" }, types.ErrInvalidAddress},
		{"empty farmer", func(m *types.MsgCreateBatch) { m.Farmer = "" }, types.ErrInvalidAddress},
		{"id lowercase", func(m *types.MsgCreateBatch) { m.Id = "ash-001" }, types.ErrInvalidBatch},
		{"id too short", func(m *types.MsgCreateBatch) { m.Id = "AB" }, types.ErrInvalidBatch},
		{"id too long", func(m *types.MsgCreateBatch) { m.Id = strings.Repeat("A", 65) }, types.ErrInvalidBatch},
		{"id with space", func(m *types.MsgCreateBatch) { m.Id = "ASH 001" }, types.ErrInvalidBatch},
		{"herb type empty", func(m *types.MsgCreateBatch) { m.HerbType = "" }, types.ErrInvalidBatch},
		{"herb type too long", func(m *types.MsgCreateBatch) { m.HerbType = strings.Repeat("h", 65) }, types.ErrInvalidBatch},
		{"lat above range", func(m *types.MsgCreateBatch) { m.OriginLatMicro = 90_000_001 }, types.ErrInvalidBatch},
		{"lat below range", func(m *types.MsgCreateBatch) { m.OriginLatMicro = -90_000_001 }, types.ErrInvalidBatch},
		{"lng above range", func(m *types.MsgCreateBatch) { m.OriginLngMicro = 180_000_001 }, types.ErrInvalidBatch},
		{"lng below range", func(m *types.MsgCreateBatch) { m.OriginLngMicro = -180_000_001 }, types.ErrInvalidBatch},
		{"harvest time zero", func(m *types.MsgCreateBatch) { m.HarvestTime = 0 }, types.ErrInvalidBatch},
		{"harvest time negative", func(m *types.MsgCreateBatch) { m.HarvestTime = -5 }, types.ErrInvalidBatch},
		{"harvest time after block time", func(m *types.MsgCreateBatch) { m.HarvestTime = testBlockTime.Unix() + 1 }, types.ErrInvalidBatch},
		{"quantity zero", func(m *types.MsgCreateBatch) { m.QuantityGrams = 0 }, types.ErrInvalidBatch},
		{"grade invalid", func(m *types.MsgCreateBatch) { m.QualityGrade = "D" }, types.ErrInvalidBatch},
		{"grade empty", func(m *types.MsgCreateBatch) { m.QualityGrade = "" }, types.ErrInvalidBatch},
		{"too many certifications", func(m *types.MsgCreateBatch) { m.Certifications = repeat("c", 11) }, types.ErrInvalidBatch},
		{"empty certification", func(m *types.MsgCreateBatch) { m.Certifications = []string{"organic", ""} }, types.ErrInvalidBatch},
		{"certification too long", func(m *types.MsgCreateBatch) { m.Certifications = []string{strings.Repeat("c", 65)} }, types.ErrInvalidBatch},
		{"location too long", func(m *types.MsgCreateBatch) { m.Location = strings.Repeat("l", 129) }, types.ErrInvalidBatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			msg := f.createMsg("ASH-001")
			tc.mutate(msg)
			_, err := f.msgServer.CreateBatch(f.ctx, msg)
			require.ErrorIs(t, err, tc.err)

			has, err := f.keeper.Batches.Has(f.ctx, msg.Id)
			require.NoError(t, err)
			require.False(t, has, "no state written on failure")
		})
	}
}

func TestCreateBatchBoundaries(t *testing.T) {
	f := initFixture(t)
	msg := f.createMsg("A-1")
	msg.OriginLatMicro, msg.OriginLngMicro = -90_000_000, 180_000_000
	msg.HarvestTime = testBlockTime.Unix()
	msg.Certifications = repeat("c", 10)
	msg.HerbType = strings.Repeat("h", 64)
	msg.Location = strings.Repeat("l", 128)
	_, err := f.msgServer.CreateBatch(f.ctx, msg)
	require.NoError(t, err)
}

func TestCreateBatchRespectsParamsMaxCertifications(t *testing.T) {
	f := initFixture(t)
	require.NoError(t, f.keeper.Params.Set(f.ctx, types.NewParams(nil, 2)))
	msg := f.createMsg("ASH-001")
	msg.Certifications = repeat("c", 3)
	_, err := f.msgServer.CreateBatch(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidBatch)
}

func TestCreateBatchCanonicalizesFarmer(t *testing.T) {
	f := initFixture(t)
	msg := f.createMsg("ASH-001")
	msg.Farmer = strings.ToUpper(f.farmer)
	_, err := f.msgServer.CreateBatch(f.ctx, msg)
	require.NoError(t, err)
	b, err := f.keeper.GetBatch(f.ctx, "ASH-001")
	require.NoError(t, err)
	require.Equal(t, f.farmer, b.Farmer)
	require.Equal(t, f.farmer, b.Owner)
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}
