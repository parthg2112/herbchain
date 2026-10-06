#!/usr/bin/env bash
# Local two-chain IBC setup for x/attest: herbchain-a <-> herbchain-b over port "attest", relayed by Hermes.
# Usage: scripts/relayer.sh {install|chains-up|chains-down|setup|start|stop|status|channel|up|down}
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${BIN:-herbchaind}"
LOG_DIR="${LOG_DIR:-$HOME/logs}"
HERMES_HOME="${HERMES_HOME:-$HOME/.hermes}"
HERMES_CONFIG="$HERMES_HOME/config.toml"
HERMES_PID="$HERMES_HOME/hermes.pid"
HERMES_VERSION="v1.13.3"
# sha256 of hermes-v1.13.3-x86_64-unknown-linux-gnu.tar.gz as published in the GitHub release asset digest.
HERMES_SHA256="3582136824ac1984c8884b310c63940a1489f091fa45faca38ed80e5dfe7d51b"
DEFAULT_HERMES_BIN="$HOME/.local/bin/hermes"
HERMES_BIN="${HERMES_BIN:-$DEFAULT_HERMES_BIN}"

PORT="attest"
VERSION="attest-1" # must equal x/attest/types.Version
KEY_NAME="relayer"
# DEV-ONLY mnemonic, also funded in chain-a.yml / chain-b.yml.
RELAYER_MNEMONIC="walk sunny stereo seven expire giraffe mom area possible august wool pudding decorate powder bunker mango lamp alter devote diary vast verb peasant endorse"

CHAINS=(a b)
declare -A CHAIN_ID=([a]=herbchain-a [b]=herbchain-b)
declare -A RPC_PORT=([a]=26557 [b]=26457)
declare -A GRPC_PORT=([a]=9080 [b]=9070)

log() { printf '[relayer] %s\n' "$*"; }
die() { printf '[relayer] ERROR: %s\n' "$*" >&2; exit 1; }

home_of() { echo "$HOME/.${CHAIN_ID[$1]}"; }
node_of() { echo "tcp://127.0.0.1:${RPC_PORT[$1]}"; }

install_hermes() {
  # The v1.13.3 release binary self-reports 1.13.2, so track the installed release with a stamp file.
  local stamp="$HERMES_BIN.$HERMES_VERSION"
  if [[ -x "$HERMES_BIN" && (-f "$stamp" || "$HERMES_BIN" != "$DEFAULT_HERMES_BIN") ]]; then
    log "hermes ${HERMES_VERSION} already at $HERMES_BIN"
    return
  fi
  local asset="hermes-${HERMES_VERSION}-x86_64-unknown-linux-gnu.tar.gz" tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"; trap - RETURN' RETURN
  log "downloading $asset"
  curl -fsSL -o "$tmp/$asset" "https://github.com/informalsystems/hermes/releases/download/${HERMES_VERSION}/${asset}"
  echo "$HERMES_SHA256  $tmp/$asset" | sha256sum -c - >/dev/null || die "checksum mismatch for $asset"
  tar -xzf "$tmp/$asset" -C "$tmp"
  mkdir -p "$(dirname "$HERMES_BIN")"
  install -m 0755 "$tmp/hermes" "$HERMES_BIN"
  touch "$stamp"
  log "installed $("$HERMES_BIN" version 2>/dev/null | head -1) at $HERMES_BIN"
}

wait_for_height() {
  local c=$1 i
  for i in $(seq 1 60); do
    if "$BIN" status --node "$(node_of "$c")" 2>/dev/null | jq -e '.sync_info.latest_block_height | tonumber > 1' >/dev/null; then
      return
    fi
    sleep 1
  done
  die "${CHAIN_ID[$c]} did not produce blocks; see $LOG_DIR/${CHAIN_ID[$c]}.log"
}

chains_up() {
  mkdir -p "$LOG_DIR"
  for c in "${CHAINS[@]}"; do
    if "$BIN" status --node "$(node_of "$c")" >/dev/null 2>&1; then
      log "${CHAIN_ID[$c]} already running"
      continue
    fi
    if [[ ! -f "$(home_of "$c")/config/genesis.json" || "${RESET:-0}" == 1 ]]; then
      log "initialising ${CHAIN_ID[$c]} from chain-$c.yml"
      (cd "$ROOT" && ignite chain init -c "chain-$c.yml" --skip-proto -y </dev/null >"$LOG_DIR/${CHAIN_ID[$c]}-init.log" 2>&1) ||
        die "ignite chain init failed; see $LOG_DIR/${CHAIN_ID[$c]}-init.log"
    fi
    log "starting ${CHAIN_ID[$c]}"
    setsid nohup "$BIN" start --home "$(home_of "$c")" >"$LOG_DIR/${CHAIN_ID[$c]}.log" 2>&1 </dev/null &
    echo $! >"$(home_of "$c")/node.pid"
  done
  for c in "${CHAINS[@]}"; do wait_for_height "$c"; done
  log "both chains producing blocks"
}

chains_down() {
  for c in "${CHAINS[@]}"; do
    local pidfile
    pidfile="$(home_of "$c")/node.pid"
    if [[ -f "$pidfile" ]] && kill "$(cat "$pidfile")" 2>/dev/null; then
      log "stopped ${CHAIN_ID[$c]}"
    fi
    rm -f "$pidfile"
  done
}

write_config() {
  mkdir -p "$HERMES_HOME"
  {
    cat <<EOF
[global]
log_level = 'info'

[mode.clients]
enabled = true
refresh = true
misbehaviour = false

[mode.connections]
enabled = false

[mode.channels]
enabled = false

[mode.packets]
enabled = true
clear_interval = 20
clear_on_start = true
tx_confirmation = true

[rest]
enabled = false
host = '127.0.0.1'
port = 3000

[telemetry]
enabled = false
host = '127.0.0.1'
port = 3001
EOF
    for c in "${CHAINS[@]}"; do
      cat <<EOF

[[chains]]
id = '${CHAIN_ID[$c]}'
type = 'CosmosSdk'
rpc_addr = 'http://127.0.0.1:${RPC_PORT[$c]}'
grpc_addr = 'http://127.0.0.1:${GRPC_PORT[$c]}'
event_source = { mode = 'push', url = 'ws://127.0.0.1:${RPC_PORT[$c]}/websocket', batch_delay = '200ms' }
rpc_timeout = '10s'
trusted_node = true
account_prefix = 'herb'
key_name = '${KEY_NAME}'
key_store_type = 'Test'
store_prefix = 'ibc'
default_gas = 200000
max_gas = 3000000
gas_price = { price = 0.025, denom = 'stake' }
gas_multiplier = 1.3
max_msg_num = 30
max_tx_size = 2097152
clock_drift = '5s'
max_block_time = '30s'
trusting_period = '14days'
trust_threshold = '2/3'
address_type = { derivation = 'cosmos' }
memo_prefix = ''

[chains.packet_filter]
policy = 'allow'
list = [['${PORT}', '*']]
EOF
    done
  } >"$HERMES_CONFIG"
  log "wrote $HERMES_CONFIG"
}

add_keys() {
  local mfile
  mfile="$(mktemp)"
  trap 'rm -f "$mfile"; trap - RETURN' RETURN
  printf '%s\n' "$RELAYER_MNEMONIC" >"$mfile"
  for c in "${CHAINS[@]}"; do
    "$HERMES_BIN" --config "$HERMES_CONFIG" keys add --chain "${CHAIN_ID[$c]}" --key-name "$KEY_NAME" \
      --mnemonic-file "$mfile" --overwrite >/dev/null 2>&1 || die "hermes keys add failed for ${CHAIN_ID[$c]}"
  done
  log "relayer key loaded on both chains"
}

# Prints the first OPEN channel id on port attest of chain $1 (empty if none).
open_channel() {
  "$BIN" q ibc channel channels --node "$(node_of "$1")" -o json 2>/dev/null |
    jq -r --arg port "$PORT" '[.channels[] | select(.port_id == $port and .state == "STATE_OPEN")][0].channel_id // empty'
}

create_channel() {
  local existing
  existing="$(open_channel a)"
  if [[ -n "$existing" ]]; then
    log "channel already open: herbchain-a/$existing"
    return
  fi
  log "creating clients, connection and channel on port $PORT"
  "$HERMES_BIN" --config "$HERMES_CONFIG" create channel \
    --a-chain "${CHAIN_ID[a]}" --b-chain "${CHAIN_ID[b]}" \
    --a-port "$PORT" --b-port "$PORT" \
    --order unordered --channel-version "$VERSION" \
    --new-client-connection --yes >"$LOG_DIR/hermes-create-channel.log" 2>&1 ||
    die "hermes create channel failed; see $LOG_DIR/hermes-create-channel.log"
  [[ -n "$(open_channel a)" && -n "$(open_channel b)" ]] || die "channel not open after handshake"
}

print_channels() {
  echo "herbchain-a $PORT/$(open_channel a) <-> herbchain-b $PORT/$(open_channel b)"
}

is_running() { [[ -f "$HERMES_PID" ]] && kill -0 "$(cat "$HERMES_PID")" 2>/dev/null; }

start_relayer() {
  if is_running; then
    log "hermes already running (pid $(cat "$HERMES_PID"))"
    return
  fi
  mkdir -p "$LOG_DIR"
  setsid nohup "$HERMES_BIN" --config "$HERMES_CONFIG" start >>"$LOG_DIR/hermes.log" 2>&1 </dev/null &
  echo $! >"$HERMES_PID"
  log "hermes started (pid $!), log $LOG_DIR/hermes.log"
}

stop_relayer() {
  if is_running; then
    kill "$(cat "$HERMES_PID")"
    log "hermes stopped"
  fi
  rm -f "$HERMES_PID"
}

main() {
  command -v jq >/dev/null || die "jq is required"
  mkdir -p "$LOG_DIR"
  case "${1:-}" in
  install) install_hermes ;;
  chains-up) chains_up ;;
  chains-down) chains_down ;;
  setup)
    install_hermes
    write_config
    add_keys
    create_channel
    print_channels
    ;;
  start) start_relayer ;;
  stop) stop_relayer ;;
  status)
    is_running && log "hermes running (pid $(cat "$HERMES_PID"))" || log "hermes not running"
    print_channels
    ;;
  channel) open_channel "${2:-a}" ;;
  up)
    chains_up
    "$0" setup
    start_relayer
    ;;
  down)
    stop_relayer
    chains_down
    ;;
  *)
    echo "usage: $0 {install|chains-up|chains-down|setup|start|stop|status|channel [a|b]|up|down}" >&2
    echo "  RESET=1 $0 chains-up   re-initialises both chains from chain-a.yml / chain-b.yml" >&2
    exit 2
    ;;
  esac
}

main "$@"
