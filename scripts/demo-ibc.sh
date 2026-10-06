#!/usr/bin/env bash
# Phase 2 demo: attest a lab-tested batch from herbchain-a to herbchain-b over the attest channel.
# Covers the success, duplicate (error ack) and timeout paths. Needs: scripts/relayer.sh up
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELAYER="$ROOT/scripts/relayer.sh"
BIN="${BIN:-herbchaind}"
A_HOME="$HOME/.herbchain-a" A_NODE="tcp://127.0.0.1:26557"
B_HOME="$HOME/.herbchain-b" B_NODE="tcp://127.0.0.1:26457"
RUN="$(date +%s)"
OK_ID="IBC-$RUN-OK"
TIMEOUT_ID="IBC-$RUN-TO"
RAW_ID="IBC-$RUN-RAW"
REPORT_HASH="$(printf 'lab-report-%s' "$RUN" | sha256sum | cut -d' ' -f1)"
FAILURES=0

step() { printf '\n== %s\n' "$*"; }
pass() { printf 'PASS: %s\n' "$*"; }
fail() {
  printf 'FAIL: %s\n' "$*"
  FAILURES=$((FAILURES + 1))
}
die() {
  printf 'FAIL: %s\n' "$*"
  exit 1
}

qa() { "$BIN" q "$@" --node "$A_NODE" -o json; }
qb() { "$BIN" q "$@" --node "$B_NODE" -o json; }

# tx_a <from> <args...>: broadcasts on chain A, waits for inclusion and prints the tx JSON.
# Fixed gas (no simulation) so msg-level rejections land in a block with their error in raw_log.
tx_a() {
  local from=$1 out hash i
  shift
  out="$("$BIN" tx "$@" --from "$from" --home "$A_HOME" --keyring-backend test --chain-id herbchain-a \
    --node "$A_NODE" --gas 500000 --yes -o json 2>&1)" || {
    echo "$out"
    return 1
  }
  hash="$(jq -r '.txhash' <<<"$out" 2>/dev/null)" || {
    echo "$out"
    return 1
  }
  [[ "$(jq -r '.code' <<<"$out")" == 0 ]] || {
    echo "$out"
    return 1
  }
  for i in $(seq 1 30); do
    if out="$(qa tx "$hash" 2>/dev/null)"; then
      echo "$out"
      [[ "$(jq -r '.code' <<<"$out")" == 0 ]]
      return
    fi
    sleep 1
  done
  echo "tx $hash not included"
  return 1
}

sent_field() { { qa attest sent-attestation "$A_CHAN" "$1" 2>/dev/null || true; } | jq -r ".sent_attestation.$2 // empty"; }

# wait_sent <batch> <status> <seconds>: polls chain A until the sent attestation reaches status.
wait_sent() {
  local batch=$1 want=$2 i got=""
  for i in $(seq 1 "$3"); do
    got="$(sent_field "$batch" status)"
    [[ "$got" == "$want" ]] && return 0
    sleep 1
  done
  echo "last status: ${got:-none}"
  return 1
}

# tested_batch <id>: creates a batch on chain A and drives it HARVESTED -> PROCESSED -> TESTED.
tested_batch() {
  local id=$1
  tx_a alice herbtrace create-batch "$id" ashwagandha "$(($(date +%s) - 3600))" 25000 A \
    --origin-lat-micro=26912400 --origin-lng-micro=75787300 --location "Jaipur farm" >/dev/null ||
    die "create-batch $id"
  tx_a alice herbtrace update-status "$id" processed "Delhi processing unit" >/dev/null || die "update-status $id"
  tx_a carol herbtrace record-lab-result "$id" true "$REPORT_HASH" >/dev/null || die "record-lab-result $id"
  [[ "$(qa herbtrace batch "$id" | jq -r '.batch.status')" =~ TESTED ]] || die "$id is not TESTED"
}

resume_relayer() { "$RELAYER" start >/dev/null; }
trap resume_relayer EXIT

step "preflight"
A_CHAN="$("$RELAYER" channel a)"
B_CHAN="$("$RELAYER" channel b)"
[[ -n "$A_CHAN" && -n "$B_CHAN" ]] || die "no open attest channel; run scripts/relayer.sh up"
"$RELAYER" start
echo "channel: herbchain-a attest/$A_CHAN <-> herbchain-b attest/$B_CHAN"

step "1. batch $OK_ID to TESTED on herbchain-a"
tested_batch "$OK_ID"
pass "$OK_ID is TESTED on herbchain-a"

step "2. sending a batch that has not passed its lab test is rejected"
tx_a alice herbtrace create-batch "$RAW_ID" tulsi "$(($(date +%s) - 3600))" 1000 B --location farm >/dev/null ||
  die "create-batch $RAW_ID"
if out="$(tx_a alice attest send-attestation "$A_CHAN" "$RAW_ID" --timeout 10m)"; then
  fail "HARVESTED batch was sent"
elif grep -q "batch has not passed lab testing" <<<"$out"; then
  pass "HARVESTED batch rejected: batch has not passed lab testing"
else
  fail "unexpected error: $out"
fi

step "3. send attestation for $OK_ID and wait for the ack"
tx_a alice attest send-attestation "$A_CHAN" "$OK_ID" --timeout 10m >/dev/null || die "send-attestation $OK_ID"
qa attest sent-attestation "$A_CHAN" "$OK_ID"
if wait_sent "$OK_ID" SENT_STATUS_ACKED 90; then pass "herbchain-a: $OK_ID ACKED"; else fail "herbchain-a: $OK_ID not ACKED"; fi

if att="$(qb attest attestation "$B_CHAN" "$OK_ID")"; then
  echo "$att"
  if [[ "$(jq -r '.attestation.status' <<<"$att")" == TESTED &&
    "$(jq -r '.attestation.lab_report_hash' <<<"$att")" == "$REPORT_HASH" ]]; then
    pass "herbchain-b: attestation for $OK_ID stored with status TESTED and matching lab hash"
  else
    fail "herbchain-b: attestation fields do not match"
  fi
else
  fail "herbchain-b: no attestation for $OK_ID"
fi

step "4. resend $OK_ID: herbchain-b rejects the duplicate with an error ack"
tx_a alice attest send-attestation "$A_CHAN" "$OK_ID" --timeout 10m >/dev/null || die "resend $OK_ID"
if wait_sent "$OK_ID" SENT_STATUS_ERRORED 90; then
  err="$(sent_field "$OK_ID" error)"
  echo "error ack: $err"
  if [[ "$err" == *"attest/1102"* ]]; then
    pass "herbchain-a: $OK_ID ERRORED with ErrDuplicateAttestation (attest/1102)"
  else
    fail "herbchain-a: unexpected error ack: $err"
  fi
  acked="$(sent_field "$OK_ID" acked_sequence)"
  if [[ -n "$acked" && "$acked" != 0 ]]; then
    pass "herbchain-a: first delivery still recorded (acked_sequence $acked)"
  else
    fail "herbchain-a: acked_sequence lost on resend"
  fi
else
  fail "herbchain-a: duplicate not ERRORED"
fi

step "5. timeout path: relayer paused, short timeout"
tested_batch "$TIMEOUT_ID"
"$RELAYER" stop
tx_a alice attest send-attestation "$A_CHAN" "$TIMEOUT_ID" --timeout 15s >/dev/null || die "send-attestation $TIMEOUT_ID"
[[ "$(sent_field "$TIMEOUT_ID" status)" == SENT_STATUS_PENDING ]] || fail "$TIMEOUT_ID not PENDING after send"
echo "waiting 25s for the timeout to elapse on herbchain-b"
sleep 25
"$RELAYER" start
if wait_sent "$TIMEOUT_ID" SENT_STATUS_TIMED_OUT 120; then
  pass "herbchain-a: $TIMEOUT_ID TIMED_OUT"
else
  fail "herbchain-a: $TIMEOUT_ID not TIMED_OUT"
fi
if qb attest attestation "$B_CHAN" "$TIMEOUT_ID" >/dev/null 2>&1; then
  fail "herbchain-b stored a timed-out attestation"
else
  pass "herbchain-b: no attestation for $TIMEOUT_ID"
fi

step "summary"
qa attest sent-attestations | jq -c --arg run "$RUN" '.sent_attestations[] | select(.batch_id | contains($run)) | {batch_id, sequence, status, error}'
if ((FAILURES > 0)); then
  echo "FAIL: $FAILURES check(s) failed"
  exit 1
fi
echo "PASS: all IBC demo checks passed"
