package types

import errorsmod "cosmossdk.io/errors"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation. Maps are used only for lookups;
// all iteration is over the genesis slices, so error reporting is deterministic.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return errorsmod.Wrap(ErrInvalidGenesis, err.Error())
	}
	batchIDs, err := gs.validateBatches()
	if err != nil {
		return err
	}
	nextSeq, err := gs.validateHistorySeqs(batchIDs)
	if err != nil {
		return err
	}
	counts, err := gs.validateHistory(batchIDs, nextSeq)
	if err != nil {
		return err
	}
	// With unique seqs all < next_seq, count == next_seq implies seqs are exactly 0..next_seq-1.
	for _, b := range gs.Batches {
		if counts[b.Id] != nextSeq[b.Id] {
			return errorsmod.Wrapf(ErrInvalidGenesis, "batch %s: %d history entries but next_seq %d (seq gap)",
				b.Id, counts[b.Id], nextSeq[b.Id])
		}
	}
	return nil
}

func (gs GenesisState) validateBatches() (map[string]struct{}, error) {
	ids := make(map[string]struct{}, len(gs.Batches))
	for _, b := range gs.Batches {
		if _, dup := ids[b.Id]; dup {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "duplicate batch id %s", b.Id)
		}
		if err := b.Validate(gs.Params.MaxCertifications); err != nil {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "batch %s: %s", b.Id, err)
		}
		ids[b.Id] = struct{}{}
	}
	return ids, nil
}

func (gs GenesisState) validateHistorySeqs(batchIDs map[string]struct{}) (map[string]uint64, error) {
	next := make(map[string]uint64, len(gs.HistorySeqs))
	for _, s := range gs.HistorySeqs {
		if _, ok := batchIDs[s.BatchId]; !ok {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "history seq for unknown batch %s", s.BatchId)
		}
		if _, dup := next[s.BatchId]; dup {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "duplicate history seq for batch %s", s.BatchId)
		}
		if s.NextSeq == 0 {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "batch %s has next_seq 0; creation entry missing", s.BatchId)
		}
		next[s.BatchId] = s.NextSeq
	}
	for _, b := range gs.Batches {
		if _, ok := next[b.Id]; !ok {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "batch %s has no history seq", b.Id)
		}
	}
	return next, nil
}

type historyKey struct {
	id  string
	seq uint64
}

func (gs GenesisState) validateHistory(batchIDs map[string]struct{}, nextSeq map[string]uint64) (map[string]uint64, error) {
	seen := make(map[historyKey]struct{}, len(gs.History))
	counts := make(map[string]uint64, len(gs.Batches))
	for _, e := range gs.History {
		if _, ok := batchIDs[e.BatchId]; !ok {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "history entry %d for unknown batch %s", e.Seq, e.BatchId)
		}
		if err := e.Validate(); err != nil {
			return nil, errorsmod.Wrap(ErrInvalidGenesis, err.Error())
		}
		key := historyKey{e.BatchId, e.Seq}
		if _, dup := seen[key]; dup {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "duplicate history entry %s/%d", e.BatchId, e.Seq)
		}
		if e.Seq >= nextSeq[e.BatchId] {
			return nil, errorsmod.Wrapf(ErrInvalidGenesis, "history entry %s/%d not below next_seq %d",
				e.BatchId, e.Seq, nextSeq[e.BatchId])
		}
		seen[key] = struct{}{}
		counts[e.BatchId]++
	}
	return counts, nil
}
