#!/usr/bin/env bash
# Phase 1 demo: walks one batch through the x/herbtrace lifecycle on a running chain
# (`make serve`), then checks queries and expected rejections. Exits non-zero on any surprise.
set -euo pipefail

BIN=${BIN:-herbchaind}
NODE=${NODE:-tcp://localhost:26657}
HOME_DIR=${HOME_DIR:-$HOME/.herbchain}
KEYRING=${KEYRING:-test}
GAS=${GAS:-400000}
FEES=${FEES:-}
TX_TIMEOUT=${TX_TIMEOUT:-30}

CODESPACE=herbtrace
ERR_TRANSITION=1104
ERR_UNAUTHORIZED=1105
ERR_TERMINAL=1106

TX_CODE="" TX_CODESPACE="" TX_LOG="" TX_HASH=""
STEP=0

log() { printf '\n== %s\n' "$*"; }
pass() { STEP=$((STEP + 1)); printf 'PASS [%02d] %s\n' "$STEP" "$*"; }
fail() { printf 'FAIL %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null || fail "missing dependency: $1"; }

addr() { "$BIN" keys show "$1" -a --keyring-backend "$KEYRING" --home "$HOME_DIR"; }

# q <args...>: herbtrace query, JSON on stdout.
q() { "$BIN" q herbtrace "$@" --node "$NODE" -o json; }

# wait_tx <hash>: poll until the tx is in a block, then set TX_CODE/TX_CODESPACE/TX_LOG.
wait_tx() {
  local hash=$1 res i
  for ((i = 0; i < TX_TIMEOUT; i++)); do
    if res=$("$BIN" q tx "$hash" --node "$NODE" -o json 2>/dev/null); then
      TX_CODE=$(jq -r '.code // 0' <<<"$res")
      TX_CODESPACE=$(jq -r '.codespace // ""' <<<"$res")
      TX_LOG=$(jq -r '.raw_log // ""' <<<"$res")
      return 0
    fi
    sleep 1
  done
  fail "tx $hash not included within ${TX_TIMEOUT}s"
}

# tx <from> <herbtrace tx args...>: sign, broadcast (sync), wait for inclusion.
tx() {
  local from=$1 out err
  shift
  local flags=(--from "$from" --chain-id "$CHAIN_ID" --keyring-backend "$KEYRING" --home "$HOME_DIR"
    --node "$NODE" --gas "$GAS" --broadcast-mode sync --yes -o json)
  [[ -n $FEES ]] && flags+=(--fees "$FEES")
  err=$(mktemp)
  if ! out=$("$BIN" tx herbtrace "$@" "${flags[@]}" 2>"$err"); then
    TX_CODE=-1 TX_CODESPACE="client" TX_LOG=$(cat "$err") TX_HASH=""
    rm -f "$err"
    return 0
  fi
  rm -f "$err"
  TX_HASH=$(jq -r '.txhash' <<<"$out")
  TX_CODE=$(jq -r '.code // 0' <<<"$out")
  if [[ $TX_CODE != 0 ]]; then # rejected in CheckTx
    TX_CODESPACE=$(jq -r '.codespace // ""' <<<"$out")
    TX_LOG=$(jq -r '.raw_log // ""' <<<"$out")
    return 0
  fi
  wait_tx "$TX_HASH"
}

# expect_ok <desc> <from> <args...>
expect_ok() {
  local desc=$1
  shift
  tx "$@"
  [[ $TX_CODE == 0 ]] || fail "$desc: code=$TX_CODE codespace=$TX_CODESPACE log=$TX_LOG"
  pass "$desc (tx ${TX_HASH:0:12})"
}

# expect_err <desc> <code> <from> <args...>: tx must be included and fail with herbtrace error <code>.
expect_err() {
  local desc=$1 code=$2
  shift 2
  tx "$@"
  [[ $TX_CODE == "$code" && $TX_CODESPACE == "$CODESPACE" ]] ||
    fail "$desc: want $CODESPACE/$code, got code=$TX_CODE codespace=$TX_CODESPACE log=$TX_LOG"
  pass "$desc -> rejected $TX_CODESPACE/$TX_CODE in block (tx ${TX_HASH:0:12}): ${TX_LOG#*message index: 0: }"
}

# expect_batch <id> <status> <owner-address>
expect_batch() {
  local b status owner
  b=$(q batch "$1")
  status=$(jq -r '.batch.status' <<<"$b")
  owner=$(jq -r '.batch.owner' <<<"$b")
  [[ $status == "$2" && $owner == "$3" ]] ||
    fail "batch $1: want status=$2 owner=$3, got status=$status owner=$owner"
  pass "batch $1 is $status, owner $owner"
}

main() {
  need "$BIN"
  need jq
  CHAIN_ID=$("$BIN" status --node "$NODE" 2>/dev/null | jq -r '.node_info.network') || fail "no node at $NODE (run: make serve)"
  local alice bob carol suffix id id2 harvest report
  alice=$(addr alice) bob=$(addr bob) carol=$(addr carol)
  suffix=$(date +%s)
  id="ASH-DEMO-$suffix"
  id2="TUL-DEMO-$suffix"
  harvest=$(($(date +%s) - 86400))
  report=$(printf 'lab-report:%s' "$id" | sha256sum | cut -d' ' -f1)

  log "chain $CHAIN_ID at $NODE"
  echo "alice (farmer)    $alice"
  echo "bob   (processor) $bob"
  echo "carol (certifier) $carol"
  q params | jq -c '.params'
  q params | jq -e --arg c "$carol" '.params.certifiers | index($c) != null' >/dev/null ||
    fail "carol is not a certifier in params"
  pass "carol is a certifier"

  log "lifecycle of $id"
  expect_ok "alice creates $id" alice create-batch "$id" ashwagandha "$harvest" 25000 A \
    --origin-lat-micro=26912400 --origin-lng-micro=75787300 --certifications organic,ayush \
    --location "Jaipur farm"
  expect_batch "$id" BATCH_STATUS_HARVESTED "$alice"
  expect_ok "alice: HARVESTED -> PROCESSED" alice update-status "$id" processed "Jaipur drying unit" \
    --remark "sun-dried, powdered"
  expect_ok "alice transfers custody to bob" alice transfer-custody "$id" "$bob" "Delhi processing unit"
  expect_batch "$id" BATCH_STATUS_PROCESSED "$bob"
  expect_ok "carol records passing lab result: PROCESSED -> TESTED" carol record-lab-result "$id" true "$report"
  expect_batch "$id" BATCH_STATUS_TESTED "$bob"
  expect_ok "bob: TESTED -> PACKAGED" bob update-status "$id" packaged "Delhi packing line"
  expect_ok "bob: PACKAGED -> SHIPPED" bob update-status "$id" shipped "NH48 in transit"
  expect_ok "bob: SHIPPED -> DELIVERED" bob update-status "$id" delivered "Mumbai distributor"
  expect_batch "$id" BATCH_STATUS_DELIVERED "$bob"
  [[ $(q batch "$id" | jq -r '.batch.lab_report_hash') == "$report" ]] || fail "lab_report_hash not stored"
  pass "lab_report_hash stored"

  log "history of $id"
  local hist actions want
  hist=$(q batch-history "$id")
  jq -r '.entries[] | [.seq // "0", .action, (.from_status // "-"), (.to_status // "-"), .actor, .location] | @tsv' <<<"$hist"
  actions=$(jq -r '[.entries[].action] | join(",")' <<<"$hist")
  want="HISTORY_ACTION_CREATED,HISTORY_ACTION_STATUS_UPDATED,HISTORY_ACTION_CUSTODY_TRANSFERRED,HISTORY_ACTION_LAB_RESULT,HISTORY_ACTION_STATUS_UPDATED,HISTORY_ACTION_STATUS_UPDATED,HISTORY_ACTION_STATUS_UPDATED"
  [[ $actions == "$want" ]] || fail "history actions: got $actions"
  # proto3 JSON omits seq 0; uint64 is rendered as a string
  jq -e '[.entries[] | .seq // "0" | tonumber] == [range(0; 7)]' <<<"$hist" >/dev/null ||
    fail "history seqs are not 0..6 in order"
  pass "history has 7 entries, seq 0..6 in order"

  log "batches-by-status delivered"
  q batches-by-status delivered | jq -r '.batches[] | [.id, .status, .owner] | @tsv'
  q batches-by-status delivered | jq -e --arg id "$id" '[.batches[].id] | index($id) != null' >/dev/null ||
    fail "$id missing from batches-by-status delivered"
  pass "$id listed under DELIVERED"
  q batches-by-owner "$bob" | jq -e --arg id "$id" '[.batches[].id] | index($id) != null' >/dev/null ||
    fail "$id missing from batches-by-owner bob"
  pass "$id listed under owner bob"

  log "expected failures (batch $id2)"
  expect_ok "alice creates $id2" alice create-batch "$id2" tulsi "$harvest" 5000 B --location "Pune farm"
  expect_err "wrong signer: bob updates alice's batch" "$ERR_UNAUTHORIZED" bob update-status "$id2" processed
  expect_err "skipped transition: HARVESTED -> PACKAGED" "$ERR_TRANSITION" alice update-status "$id2" packaged
  expect_ok "alice: HARVESTED -> PROCESSED" alice update-status "$id2" processed
  expect_err "non-certifier lab result: bob" "$ERR_UNAUTHORIZED" bob record-lab-result "$id2" true "$report"
  expect_batch "$id2" BATCH_STATUS_PROCESSED "$alice"
  expect_err "terminal state: update after DELIVERED" "$ERR_TERMINAL" bob update-status "$id" processed

  log "all $STEP checks passed"
}

main "$@"
