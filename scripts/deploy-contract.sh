#!/usr/bin/env bash
# deploy-contract.sh – Build, deploy, and configure the DiasporaConnect Anchor program.
#
# Usage:
#   ./scripts/deploy-contract.sh [cluster]
#
# Arguments:
#   cluster   Solana cluster to deploy to. Supported values:
#               localnet  (default) – local test validator
#               devnet    – Solana devnet
#               mainnet   – Solana mainnet-beta (use with caution!)
#
# Required environment variables:
#   TREASURY_PUBLIC_KEY   Base58 public key of the treasury token account.
#                         All 1% fees are routed here.
#
# Optional environment variables:
#   WALLET_PATH           Path to the Solana keypair JSON file.
#                         Default: ~/.config/solana/id.json
#   ANCHOR_VERSION        Anchor CLI version to verify (informational only).
#
# Output (written to stdout AND to .contract-info.json):
#   program_id            Deployed program ID (base58).
#   cluster               Target cluster.
#   treasury_public_key   Treasury public key used.
#   idl_path              Path to the generated IDL JSON.
#   deployed_at           ISO-8601 timestamp of the deployment.
#
# The .contract-info.json file can be sourced by the Go backend at startup to
# populate SOLANA_PROGRAM_ID and TREASURY_PUBLIC_KEY automatically.
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

CLUSTER="${1:-localnet}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ANCHOR_DIR="${REPO_ROOT}/anchor-diasporaconnect"
INFO_FILE="${REPO_ROOT}/.contract-info.json"

# ── Colour helpers ────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ── Validate prerequisites ───────────────────────────────────────────────────
command -v anchor  >/dev/null 2>&1 || error "anchor CLI not found. Install with: cargo install --git https://github.com/coral-xyz/anchor avm --locked"
command -v solana  >/dev/null 2>&1 || error "solana CLI not found. Install via https://docs.solana.com/cli/install-solana-cli-tools"
command -v jq      >/dev/null 2>&1 || error "jq not found. Install with: apt-get install jq"

info "Anchor version: $(anchor --version)"
info "Solana version: $(solana --version)"

# ── Validate env vars ────────────────────────────────────────────────────────
if [[ -z "${TREASURY_PUBLIC_KEY:-}" ]]; then
    error "TREASURY_PUBLIC_KEY environment variable is required."
fi

WALLET_PATH="${WALLET_PATH:-${HOME}/.config/solana/id.json}"
if [[ ! -f "${WALLET_PATH}" ]]; then
    warn "No wallet found at ${WALLET_PATH}. Generating a new keypair..."
    solana-keygen new --no-passphrase --outfile "${WALLET_PATH}"
fi

info "Wallet: ${WALLET_PATH}"
info "Cluster: ${CLUSTER}"
info "Treasury public key: ${TREASURY_PUBLIC_KEY}"

# ── Map cluster alias to Solana RPC URL ──────────────────────────────────────
case "${CLUSTER}" in
    localnet)   RPC_URL="http://localhost:8899" ;;
    devnet)     RPC_URL="https://api.devnet.solana.com" ;;
    testnet)    RPC_URL="https://api.testnet.solana.com" ;;
    mainnet)    RPC_URL="https://api.mainnet-beta.solana.com"
                warn "Deploying to MAINNET. Press Ctrl-C within 5 seconds to abort."
                sleep 5 ;;
    *)          error "Unknown cluster '${CLUSTER}'. Use: localnet | devnet | testnet | mainnet" ;;
esac

solana config set --url "${RPC_URL}" --keypair "${WALLET_PATH}" >/dev/null

# ── Patch TREASURY_AUTHORITY into the Rust source ───────────────────────────
LIB_RS="${ANCHOR_DIR}/programs/diasporaconnect/src/lib.rs"
info "Patching TREASURY_AUTHORITY in lib.rs ..."
# Replace the placeholder with the real value (works on both GNU and BSD sed)
sed -i.bak "s|pub const TREASURY_AUTHORITY: &str = \".*\";|pub const TREASURY_AUTHORITY: \&str = \"${TREASURY_PUBLIC_KEY}\";|g" "${LIB_RS}"
rm -f "${LIB_RS}.bak"

# ── Build ─────────────────────────────────────────────────────────────────────
info "Building Anchor program ..."
cd "${ANCHOR_DIR}"
anchor build 2>&1

# ── Read program ID from the compiled keypair ────────────────────────────────
KEYPAIR_PATH="${ANCHOR_DIR}/target/deploy/diaspora_connect-keypair.json"
if [[ ! -f "${KEYPAIR_PATH}" ]]; then
    error "Build keypair not found at ${KEYPAIR_PATH}. Did 'anchor build' succeed?"
fi
PROGRAM_ID="$(solana-keygen pubkey "${KEYPAIR_PATH}")"
info "Program ID: ${PROGRAM_ID}"

# Patch declare_id! in lib.rs so it matches the actual deployment keypair.
sed -i.bak "s|declare_id!(\".*\")|declare_id!(\"${PROGRAM_ID}\")|g" "${LIB_RS}"
rm -f "${LIB_RS}.bak"

# Patch Anchor.toml with the correct program address for the target cluster.
TOML="${ANCHOR_DIR}/Anchor.toml"
sed -i.bak "s|DiasporaConnect = \".*\"|DiasporaConnect = \"${PROGRAM_ID}\"|g" "${TOML}"
rm -f "${TOML}.bak"

# ── Rebuild with the correct program ID ──────────────────────────────────────
info "Rebuilding with patched program ID ..."
anchor build 2>&1

# ── Deploy ────────────────────────────────────────────────────────────────────
info "Deploying program to ${CLUSTER} ..."
anchor deploy --program-keypair "${KEYPAIR_PATH}" 2>&1

IDL_PATH="${ANCHOR_DIR}/target/idl/diaspora_connect.json"

# ── Write contract info ───────────────────────────────────────────────────────
DEPLOYED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

cat > "${INFO_FILE}" <<JSON
{
  "program_id": "${PROGRAM_ID}",
  "cluster": "${CLUSTER}",
  "rpc_url": "${RPC_URL}",
  "treasury_public_key": "${TREASURY_PUBLIC_KEY}",
  "idl_path": "${IDL_PATH}",
  "deployed_at": "${DEPLOYED_AT}"
}
JSON

echo ""
echo "════════════════════════════════════════════════════════════════"
echo -e "${GREEN}Deployment successful!${NC}"
echo "────────────────────────────────────────────────────────────────"
echo "  Program ID:           ${PROGRAM_ID}"
echo "  Cluster:              ${CLUSTER}"
echo "  RPC URL:              ${RPC_URL}"
echo "  Treasury public key:  ${TREASURY_PUBLIC_KEY}"
echo "  IDL:                  ${IDL_PATH}"
echo "  Deployed at:          ${DEPLOYED_AT}"
echo "────────────────────────────────────────────────────────────────"
echo "  Contract info saved → ${INFO_FILE}"
echo ""
echo "  Set the following environment variables in your Go backend:"
echo "    export SOLANA_PROGRAM_ID=${PROGRAM_ID}"
echo "    export SOLANA_RPC_URL=${RPC_URL}"
echo "    export TREASURY_PUBLIC_KEY=${TREASURY_PUBLIC_KEY}"
echo "════════════════════════════════════════════════════════════════"
