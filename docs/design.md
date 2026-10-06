# HerbChain design notes

This document records the design decisions behind `x/herbtrace` and `x/attest`. The README covers usage.

## 1. State: explicit KeySet indexes vs `collections.IndexedMap`

<!-- TODO(orchestrator): replace this section with the herbtrace agent's final choice and rationale. -->

Current code (`x/herbtrace/keeper/keeper.go`, `batch.go`) uses a plain `Map[string, Batch]` plus two `KeySet` indexes, `OwnerIndex` (`Pair[AccAddress, id]`) and `StatusIndex` (`Pair[int32, id]`). Every batch write goes through one function, `setBatch(old, new)`. It removes stale index entries only when the owner or status actually changed, then writes the new entries.

Trade-offs to state here:
- `IndexedMap` keeps the indexes consistent automatically. Its cost is a reference-key layout fixed by the library. It also does an index lookup-and-diff on every `Set`, even when nothing indexed changed.
- Explicit KeySets make the key layout and the write set visible in the code under review. The cost is discipline: any write that bypasses `setBatch` corrupts the indexes. Tests must cover index state after create, status change and custody transfer.

## 2. Coordinates as `int64` microdegrees, not floats

`origin_lat_micro` and `origin_lng_micro` store degrees × 10⁶, range-checked to ±90·10⁶ and ±180·10⁶. One microdegree is about 0.11 m at the equator, which is finer than any farm-plot use case.

Reasons for no floats in state:
- **Consensus.** The Go spec lets the compiler fuse `x*y + z` into one FMA instruction. That happens on arm64, ppc64le and s390x but not by default on amd64. Validators on different architectures could compute different results and halt the chain.
- **Encoding.** Floats have several representations of "the same" value (`-0`/`+0`, NaN payloads, decimal round-trip in JSON sign bytes). Integers have exactly one.
- **Validation.** Range checks on integers are exact. No epsilon policy is needed.

The CLI and clients convert at the edge. `--origin-lat-micro=26912400` means 26.9124°.

## 3. Time

`harvest_time` is unix seconds supplied by the farmer and must be no later than the block time. `HistoryEntry.time` and every `*_height` field come from `sdk.UnwrapSDKContext(ctx).BlockTime()` and `BlockHeight()`. Nothing in `x/` reads the wall clock. `.golangci.yml` enforces this with a `forbidigo` rule against `time.Now` in `x/*/keeper` and `x/*/types`.

## 4. Certifiers in params vs a role module

The certifiers are `params.certifiers`, a list of canonical bech32 addresses changed by `MsgUpdateParams` (gov authority). `Params.Validate` rejects bad and duplicate addresses. It also rejects non-canonical ones, so the membership check is an exact string compare over a short, gov-bounded slice.

Why not a role module (custom RBAC, `x/group` or `x/authz`):
- There is one role with one permission. A role module adds a second state machine, genesis and migrations for no new behaviour.
- Who certifies herbs is a chain-level policy decision. Governance is the right authority for it, and params already have the standard gov-authority update path.
- The cost is accepted: adding or removing a certifier needs a proposal. There is no delegation, scoping by herb type or expiry. If those are needed, the `isCertifier` check is the only call site to swap.

## 5. Indexes are rebuilt at genesis, not exported

`ExportGenesis` writes params, batches, history and seqs. `InitGenesis` calls `setBatch(nil, b)` for each batch, which rebuilds `OwnerIndex` and `StatusIndex`.

- Indexes are derived data. Exporting them would let a genesis file carry an index that disagrees with `Batch.owner` or `Batch.status`. `Validate` would then have to cross-check them.
- The index key layout stays private. Changing it (a new index, a different key codec) needs no genesis migration.
- Genesis stays smaller and diffable: one record per fact.

`GenesisState.Validate` rejects duplicate batch ids, invalid batch fields, history entries for unknown batches, seq gaps (entries must be `0..next_seq-1`), a seq record without a batch, and invalid params.

## 6. Status transitions

- One pure function, `types.ValidateTransition(from, to)`, holds every legal edge. It is table-tested over all from/to pairs, including UNSPECIFIED and out-of-range values.
- The question of who may drive a transition is separate. `IsOwnerDrivenStatus` covers PROCESSED, PACKAGED, SHIPPED and DELIVERED, which go through `MsgUpdateStatus`. `IsLabStatus` covers TESTED and REJECTED, which go only through `MsgRecordLabResult`. An owner therefore cannot self-certify, even when the edge itself is legal.
- Check order in `MsgUpdateStatus` is: signer address, batch exists, signer is owner, batch not terminal, target is owner-driven, then `ValidateTransition`. The error code tells a client exactly which rule failed: `ErrUnauthorized` 1105, `ErrTerminalStatus` 1106 or `ErrInvalidTransition` 1104.
- REJECTED is reachable only from a failed lab test and is terminal. There is no owner-driven "discard".
- Custody transfer is orthogonal to status. It is allowed in any non-terminal state and is recorded as its own history action.

## 7. History

Each batch has its own sequence: `History[Pair[id, seq]]` with `HistorySeq[id]` as the next seq. A prefix range over `id` returns entries in seq order, and `BatchHistory` paginates them. History is explicit state because a Cosmos node has no per-key history API, and pruning nodes discard old versions.

## 8. Determinism rules

- Time and height come only from the block context (§3).
- No floats in state or in state-transition arithmetic (§2).
- No Go map iteration in state transitions. Membership checks scan slices. Exports walk collections in key order, so `ExportGenesis` output is byte-stable.
- No goroutines in keeper or msg-server code.
- Every user-facing failure is a registered module error (`errorsmod.Register`, codespace `herbtrace`, codes 1100–1110), wrapped with context. Panics happen only at app wiring (`NewKeeper` with a bad authority), never on msg input.
- All state goes through `cosmossdk.io/collections`. There is no hand-built prefix handling.

## 9. IBC attestation (`x/attest`)

<!-- TODO(orchestrator): confirm each point below against the IBC agent's final keeper and IBC module callbacks. -->

- **Packet.** `AttestationPacket{batch_id, herb_type, farmer, status, lab_report_hash, source_height}` on port `attest`, unordered channel. The ack is `{received: bool}`. Failures use the standard error acknowledgement.
- **Send.** `MsgSendAttestation` reads the batch through an expected-keeper interface (`GetBatch`). It rejects any status outside TESTED, PACKAGED, SHIPPED and DELIVERED, so only batches that passed a lab test are attested. The packet carries the sender chain's state at `source_height`. It is a snapshot, not a live link.
- **Status as string.** Chain B decodes the status without importing chain A's enum, so the two chains can upgrade `x/herbtrace` independently. The receiver validates the string against its known set.
- **Receiver key.** `Attestations[Pair[channel, batch_id]]` uses the local (destination) channel. A channel end maps 1:1 to its counterparty, so two source chains that reuse a batch id never collide. A second packet for the same pair is rejected with an error ack, and the receiver's state does not change.
- **Sender outcome.** `SentAttestations[Pair[channel, batch_id]]` is set to PENDING on send. `OnAcknowledgementPacket` changes it to ACKED, or to ERRORED with the error string. `OnTimeoutPacket` changes it to TIMED_OUT.
- **Trust.** IBC light clients prove that the packet came from chain A's `attest` module. They do not prove that the lab result is true. Chain B trusts chain A's certifier set.

## 10. Differences from the Fabric chaincode

Source: `asset-transfer-basic/chaincode-javascript/lib/herbTraceability.js`.

| Concern | Fabric chaincode | HerbChain |
|---|---|---|
| Timestamps | `new Date().toISOString()` in `InitLedger`, `createBatch` and `updateBatch`. Each endorsing peer computes its own value, so read-write sets diverge under a multi-org endorsement policy. | Block header time and height only. |
| Access control | None. Any identity can call `updateBatch` on any batch. `updatedBy` is a caller-supplied string, and `ctx.clientIdentity` is never read. | Signer is taken from the tx signature. Only the owner moves status or custody. Only `params.certifiers` record lab results. |
| Status | Any string is accepted. The seed data even uses `'Processing'`, which no flow produces. | Closed enum, `ValidateTransition`, terminal states. |
| Farmer / origin | Free strings (`'Ramesh Kumar'`, `'Rajasthan, India'`). | `farmer` is the signer's bech32 address. Origin is range-checked microdegrees. |
| Input validation | `parseInt(quantity)` can store `NaN`. `certifications` takes any JSON with no size cap. There is no id or grade check. | Every field is validated (README, brief §3). Errors are registered and coded. |
| History | `previousHash` is not a hash. It is base64 of the previous record, *including that record's `previousHash`*, so each update grows the batch by more than 4/3×. History reads use `getHistoryForKey`, which depends on the peer's history DB and returns whole snapshots. | Fixed-size `HistoryEntry` per change, keyed `(id, seq)`, with the actor, action, from/to status and block time. Paginated. |
| Listing | `getAllBatches` runs an unbounded `getStateByRange('', '')` over the whole namespace, including `TEST_KEY` written by the public `testStore`. | `query.CollectionPaginate` on every list. No test hooks in the msg surface. |
| Secondary queries | CouchDB rich queries (`queryBatchesByHerbType`, `queryBatchesByFarmer`). They are not re-checked at commit, so phantom reads are possible, and they need CouchDB. | Explicit `OwnerIndex` and `StatusIndex`, kept in the same state commitment. Herb type and farmer are not indexed. |
| Seeding | `InitLedger` is a public transaction. Anyone can call it at any time to overwrite `ASHWA-2025-09-07-001`, because `putState` has no existence check. | Initial data comes only from genesis. `MsgCreateBatch` rejects existing ids. |
| Lab results / custody | Not modelled. | `MsgRecordLabResult`, which stores the report sha256, and `MsgTransferCustody`, which updates the owner index. |
| Events | None. | Typed events for every state change. |
| Cross-org sharing | Same channel only. | Attestations to another chain over IBC (`x/attest`). |
