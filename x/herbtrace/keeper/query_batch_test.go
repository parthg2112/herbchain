package keeper_test

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func requireCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, status.Code(err), err.Error())
}

func TestQueryBatch(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ASH-001", types.StatusHarvested)

	res, err := f.queryServer.Batch(f.ctx, &types.QueryBatchRequest{Id: "ASH-001"})
	require.NoError(t, err)
	require.Equal(t, "ASH-001", res.Batch.Id)

	_, err = f.queryServer.Batch(f.ctx, nil)
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.Batch(f.ctx, &types.QueryBatchRequest{})
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.Batch(f.ctx, &types.QueryBatchRequest{Id: "NOPE-1"})
	requireCode(t, err, codes.NotFound)
}

func TestQueryParams(t *testing.T) {
	f := initFixture(t)
	res, err := f.queryServer.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, types.NewParams([]string{f.certifier}, types.DefaultMaxCertifications), res.Params)

	_, err = f.queryServer.Params(f.ctx, nil)
	requireCode(t, err, codes.InvalidArgument)
}

// paginate walks every page with next_key and returns ids plus the number of pages.
func paginate(t *testing.T, limit uint64, fetch func(*query.PageRequest) ([]string, *query.PageResponse, error)) ([]string, int) {
	t.Helper()
	var (
		ids   []string
		pages int
		key   []byte
	)
	for {
		got, page, err := fetch(&query.PageRequest{Key: key, Limit: limit})
		require.NoError(t, err)
		require.LessOrEqual(t, uint64(len(got)), limit)
		ids = append(ids, got...)
		pages++
		if len(page.NextKey) == 0 {
			return ids, pages
		}
		key = page.NextKey
	}
}

func TestQueryBatchesPagination(t *testing.T) {
	f := initFixture(t)
	want := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("ASH-%03d", i)
		f.createAt(t, id, types.StatusHarvested)
		want = append(want, id)
	}

	ids, pages := paginate(t, 2, func(p *query.PageRequest) ([]string, *query.PageResponse, error) {
		res, err := f.queryServer.Batches(f.ctx, &types.QueryBatchesRequest{Pagination: p})
		if err != nil {
			return nil, nil, err
		}
		return batchIDs(res.Batches), res.Pagination, nil
	})
	require.Equal(t, want, ids)
	require.Equal(t, 3, pages)

	res, err := f.queryServer.Batches(f.ctx, &types.QueryBatchesRequest{
		Pagination: &query.PageRequest{Offset: 3, Limit: 10, CountTotal: true},
	})
	require.NoError(t, err)
	require.Equal(t, want[3:], batchIDs(res.Batches))
	require.Equal(t, uint64(5), res.Pagination.Total)

	_, err = f.queryServer.Batches(f.ctx, &types.QueryBatchesRequest{Pagination: &query.PageRequest{Offset: 1, Key: []byte("x")}})
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.Batches(f.ctx, nil)
	requireCode(t, err, codes.InvalidArgument)
}

func TestQueryBatchHistory(t *testing.T) {
	f := initFixture(t)
	// "ASH" is a byte prefix of "ASH1"; the pair-key encoding must not leak entries across ids.
	f.createAt(t, "ASH", types.StatusDelivered)
	f.createAt(t, "ASH1", types.StatusTested)

	all := f.history(t, "ASH")
	require.Len(t, all, 6)
	wantActions := []types.HistoryAction{
		types.ActionCreated, types.ActionStatusUpdated, types.ActionLabResult,
		types.ActionStatusUpdated, types.ActionStatusUpdated, types.ActionStatusUpdated,
	}
	for i, e := range all {
		require.Equal(t, "ASH", e.BatchId)
		require.Equal(t, uint64(i), e.Seq)
		require.Equal(t, wantActions[i], e.Action)
	}
	require.Len(t, f.history(t, "ASH1"), 3)

	var seqs []string
	_, pages := paginate(t, 4, func(p *query.PageRequest) ([]string, *query.PageResponse, error) {
		res, err := f.queryServer.BatchHistory(f.ctx, &types.QueryBatchHistoryRequest{Id: "ASH", Pagination: p})
		if err != nil {
			return nil, nil, err
		}
		for _, e := range res.Entries {
			seqs = append(seqs, fmt.Sprint(e.Seq))
		}
		return make([]string, len(res.Entries)), res.Pagination, nil
	})
	require.Equal(t, 2, pages)
	require.Equal(t, []string{"0", "1", "2", "3", "4", "5"}, seqs)

	res, err := f.queryServer.BatchHistory(f.ctx, &types.QueryBatchHistoryRequest{
		Id: "ASH", Pagination: &query.PageRequest{Limit: 2, Reverse: true},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(5), res.Entries[0].Seq)

	_, err = f.queryServer.BatchHistory(f.ctx, nil)
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.BatchHistory(f.ctx, &types.QueryBatchHistoryRequest{})
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.BatchHistory(f.ctx, &types.QueryBatchHistoryRequest{Id: "NOPE-1"})
	requireCode(t, err, codes.NotFound)
	_, err = f.queryServer.BatchHistory(f.ctx, &types.QueryBatchHistoryRequest{
		Id: "ASH", Pagination: &query.PageRequest{Offset: 1, Key: []byte("x")},
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestQueryBatchesByOwner(t *testing.T) {
	f := initFixture(t)
	for i := 0; i < 5; i++ {
		f.createAt(t, fmt.Sprintf("OWN-%d", i), types.StatusHarvested)
	}
	for _, id := range []string{"OWN-1", "OWN-3"} {
		_, err := f.msgServer.TransferCustody(f.ctx, &types.MsgTransferCustody{Owner: f.farmer, BatchId: id, NewOwner: f.processor})
		require.NoError(t, err)
	}

	byOwner := func(owner string) func(*query.PageRequest) ([]string, *query.PageResponse, error) {
		return func(p *query.PageRequest) ([]string, *query.PageResponse, error) {
			res, err := f.queryServer.BatchesByOwner(f.ctx, &types.QueryBatchesByOwnerRequest{Owner: owner, Pagination: p})
			if err != nil {
				return nil, nil, err
			}
			return batchIDs(res.Batches), res.Pagination, nil
		}
	}
	ids, pages := paginate(t, 2, byOwner(f.farmer))
	require.Equal(t, []string{"OWN-0", "OWN-2", "OWN-4"}, ids)
	require.Equal(t, 2, pages)
	ids, _ = paginate(t, 1, byOwner(f.processor))
	require.Equal(t, []string{"OWN-1", "OWN-3"}, ids)
	require.Empty(t, f.idsByOwner(t, f.stranger))

	_, err := f.queryServer.BatchesByOwner(f.ctx, nil)
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.BatchesByOwner(f.ctx, &types.QueryBatchesByOwnerRequest{Owner: "bad"})
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.BatchesByOwner(f.ctx, &types.QueryBatchesByOwnerRequest{
		Owner: f.farmer, Pagination: &query.PageRequest{Offset: 1, Key: []byte("x")},
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestQueryBatchesByStatus(t *testing.T) {
	f := initFixture(t)
	f.createAt(t, "ST-A", types.StatusHarvested)
	f.createAt(t, "ST-B", types.StatusProcessed)
	f.createAt(t, "ST-C", types.StatusProcessed)
	f.createAt(t, "ST-D", types.StatusProcessed)
	f.createAt(t, "ST-E", types.StatusRejected)

	ids, pages := paginate(t, 2, func(p *query.PageRequest) ([]string, *query.PageResponse, error) {
		res, err := f.queryServer.BatchesByStatus(f.ctx, &types.QueryBatchesByStatusRequest{
			Status: types.StatusProcessed, Pagination: p,
		})
		if err != nil {
			return nil, nil, err
		}
		return batchIDs(res.Batches), res.Pagination, nil
	})
	require.Equal(t, []string{"ST-B", "ST-C", "ST-D"}, ids)
	require.Equal(t, 2, pages)
	require.Equal(t, []string{"ST-A"}, f.idsByStatus(t, types.StatusHarvested))
	require.Equal(t, []string{"ST-E"}, f.idsByStatus(t, types.StatusRejected))
	require.Empty(t, f.idsByStatus(t, types.StatusDelivered))

	for _, s := range []types.BatchStatus{types.StatusUnspecified, 99} {
		_, err := f.queryServer.BatchesByStatus(f.ctx, &types.QueryBatchesByStatusRequest{Status: s})
		requireCode(t, err, codes.InvalidArgument)
	}
	_, err := f.queryServer.BatchesByStatus(f.ctx, nil)
	requireCode(t, err, codes.InvalidArgument)
	_, err = f.queryServer.BatchesByStatus(f.ctx, &types.QueryBatchesByStatusRequest{
		Status: types.StatusHarvested, Pagination: &query.PageRequest{Offset: 1, Key: []byte("x")},
	})
	requireCode(t, err, codes.InvalidArgument)
}
