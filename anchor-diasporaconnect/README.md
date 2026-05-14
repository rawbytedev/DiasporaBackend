# DiasporaConnect Anchor Smart Contract

This Anchor program implements the DiasporaConnect escrow flow for Solana USDT transfers.

## What it does
- `initiate_transfer`: locks USDT into a PDA escrow and stores sender, recipient, amount, fee, expiration and status.
- `claim_transfer`: allows the recipient to claim funds before expiration, sends 99% to recipient and 1% fee to treasury.
- `refund_transfer`: allows the sender to refund the escrow after 7 days if the transfer was not claimed.

## Files
- `Anchor.toml` — Anchor program config
- `programs/diasporaconnect/Cargo.toml` — Rust package manifest
- `programs/diasporaconnect/src/lib.rs` — smart contract code

## Testing
You can run this in a cloud environment such as GitHub Codespaces, Gitpod, or Replit if you install the Solana and Anchor toolchain.

### Local test flow
1. Install Solana CLI and Anchor CLI.
2. Start local validator:
   ```bash
   solana-test-validator
   ```
3. In another shell:
   ```bash
   cd anchor-diasporaconnect
   anchor build
   anchor deploy --provider.cluster localnet
   ```

### Devnet deployment
1. Configure provider cluster in `Anchor.toml`:
   ```toml
   [provider]
   cluster = "devnet"
   ```
2. Deploy:
   ```bash
   anchor deploy --provider.cluster devnet
   ```

### Online development / testing
- Use `https://replit.com/` or `https://gitpod.io/` to create a cloud Linux environment with Rust, Solana CLI and Anchor installed.
- You can inspect Devnet transactions at:
  - `https://explorer.solana.com?cluster=devnet`
  - `https://solscan.io?cluster=devnet`

## Notes
- Identity mapping remains off-chain: the contract only holds escrow state and funds.
- The PDA escrow is derived from sender, recipient and nonce.
- Fees are collected automatically when a transfer is claimed.
