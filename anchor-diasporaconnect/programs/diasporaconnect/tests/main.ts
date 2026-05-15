import * as anchor from "@coral-xyz/anchor";
import { Program } from "@coral-xyz/anchor";
import { DiasporaConnect } from "../target/types/diaspora_connect";
import { PublicKey, Keypair, LAMPORTS_PER_SOL } from "@solana/web3.js";
import {
  TOKEN_PROGRAM_ID,
  getOrCreateAssociatedTokenAccount,
  createMint,
  mintTo,
} from "@solana/spl-token";
import assert from "assert";

describe("diaspora_connect", () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);

  const program = anchor.workspace.DiasporaConnect as Program<DiasporaConnect>;

  let sender: Keypair;
  let recipient: Keypair;
  let treasuryAuthority: Keypair;
  let feeTreasury: PublicKey;
  let mint: PublicKey;
  let senderTokenAccount: PublicKey;
  let recipientTokenAccount: PublicKey;

  const initialBalance = 100_000_000;
  const transferAmount = 10_000_000;
  const feeAmount = transferAmount / 100;

  before(async () => {
    sender = Keypair.generate();
    recipient = Keypair.generate();
    treasuryAuthority = Keypair.generate();

    const sleep = (ms: number) =>
      new Promise((resolve) => setTimeout(resolve, ms));
    // Airdrop SOL
    console.log("advancing");
    const airdrop = async (pubkey: PublicKey, amount: number) => {
      const tx = await provider.connection.requestAirdrop(pubkey, amount);
      await provider.connection.confirmTransaction(tx);
    };
    await airdrop(sender.publicKey, 1 * LAMPORTS_PER_SOL);
    await sleep(1000);
    await airdrop(recipient.publicKey, 0.5 * LAMPORTS_PER_SOL);
    await sleep(1000);
    await airdrop(treasuryAuthority.publicKey, 2 * LAMPORTS_PER_SOL);
    console.log("passed airdrop");
    // Create mint (0 decimals for simplicity)
    mint = await createMint(
      provider.connection,
      sender,
      sender.publicKey,
      null,
      0
    );

    // Create token accounts
    senderTokenAccount = (
      await getOrCreateAssociatedTokenAccount(
        provider.connection,
        sender,
        mint,
        sender.publicKey
      )
    ).address;
    recipientTokenAccount = (
      await getOrCreateAssociatedTokenAccount(
        provider.connection,
        recipient,
        mint,
        recipient.publicKey
      )
    ).address;
    feeTreasury = (
      await getOrCreateAssociatedTokenAccount(
        provider.connection,
        treasuryAuthority,
        mint,
        treasuryAuthority.publicKey
      )
    ).address;

    // Mint tokens to sender
    await mintTo(
      provider.connection,
      sender,
      mint,
      senderTokenAccount,
      sender,
      initialBalance
    );

    // Important: Update the TREASURY_AUTHORITY constant in lib.rs to match treasuryAuthority.publicKey
    // For testing, you can either manually replace the constant or use a build script.
    // Here we assume you have done that before building.
  });

  it("Allows a user to initiate a transfer", async () => {
    const nonce = new anchor.BN(Date.now());
    const [escrowPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-escrow"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );
    const [vaultPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-vault"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );

    const before = (
      await provider.connection.getTokenAccountBalance(senderTokenAccount)
    ).value.amount;

    await program.methods
      .initiateTransfer(new anchor.BN(transferAmount), nonce)
      .accounts({
        sender: sender.publicKey,
        senderTokenAccount,
        recipient: recipient.publicKey,
        feeTreasury,
        mint,
        escrowAccount: escrowPda,
        escrowVault: vaultPda,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .signers([sender])
      .rpc();

    const after = (
      await provider.connection.getTokenAccountBalance(senderTokenAccount)
    ).value.amount;
    assert.equal(Number(after), Number(before) - transferAmount);

    const escrow = await program.account.escrowAccount.fetch(escrowPda);
    assert.equal(escrow.sender.toBase58(), sender.publicKey.toBase58());
    assert.equal(escrow.amount.toNumber(), transferAmount);
    assert.deepEqual(escrow.status, { pending: {} });
  });

  it("Fails to initiate with zero amount", async () => {
    const nonce = new anchor.BN(Date.now() + 1);
    const [escrowPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-escrow"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );
    const [vaultPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-vault"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );

    try {
      await program.methods
        .initiateTransfer(new anchor.BN(0), nonce)
        .accounts({
          sender: sender.publicKey,
          senderTokenAccount,
          recipient: recipient.publicKey,
          feeTreasury,
          mint,
          escrowAccount: escrowPda,
          escrowVault: vaultPda,
          tokenProgram: TOKEN_PROGRAM_ID,
          systemProgram: anchor.web3.SystemProgram.programId,
          rent: anchor.web3.SYSVAR_RENT_PUBKEY,
        })
        .signers([sender])
        .rpc();
      assert.fail("Should have failed");
    } catch (err) {
      assert.ok(err.message.includes("Invalid transfer amount"));
    }
  });

  it("Fails to initiate self‑transfer", async () => {
    const nonce = new anchor.BN(Date.now() + 2);
    try {
      const [escrowPda] = PublicKey.findProgramAddressSync(
        [
          Buffer.from("diaspora-escrow"),
          sender.publicKey.toBuffer(),
          sender.publicKey.toBuffer(),
          nonce.toBuffer("le", 8),
        ],
        program.programId
      );
      const [vaultPda] = PublicKey.findProgramAddressSync(
        [
          Buffer.from("diaspora-vault"),
          sender.publicKey.toBuffer(),
          sender.publicKey.toBuffer(),
          nonce.toBuffer("le", 8),
        ],
        program.programId
      );
      await program.methods
        .initiateTransfer(new anchor.BN(transferAmount), nonce)
        .accounts({
          sender: sender.publicKey,
          senderTokenAccount,
          recipient: sender.publicKey,
          feeTreasury,
          mint,
          escrowAccount: escrowPda,
          escrowVault: vaultPda,
          tokenProgram: TOKEN_PROGRAM_ID,
          systemProgram: anchor.web3.SystemProgram.programId,
          rent: anchor.web3.SYSVAR_RENT_PUBKEY,
        })
        .signers([sender])
        .rpc();
      assert.fail("Should have failed");
    } catch (err) {
      assert.ok(err.message, "Sender and recipient cannot be the same");
    }
  });

  it("Allows recipient to claim and closes vault", async () => {
    const nonce = new anchor.BN(Date.now() + 100);
    const [escrowPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-escrow"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );
    const [vaultPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-vault"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );

    await mintTo(
      provider.connection,
      sender,
      mint,
      senderTokenAccount,
      sender,
      transferAmount
    );

    await program.methods
      .initiateTransfer(new anchor.BN(transferAmount), nonce)
      .accounts({
        sender: sender.publicKey,
        senderTokenAccount,
        recipient: recipient.publicKey,
        feeTreasury,
        mint,
        escrowAccount: escrowPda,
        escrowVault: vaultPda,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .signers([sender])
      .rpc();

    const recipientBefore = (
      await provider.connection.getTokenAccountBalance(recipientTokenAccount)
    ).value.amount;
    const treasuryBefore = (
      await provider.connection.getTokenAccountBalance(feeTreasury)
    ).value.amount;

    await program.methods
      .claimTransfer(nonce)
      .accounts({
        recipient: recipient.publicKey,
        recipientTokenAccount,
        sender: sender.publicKey,
        escrowAccount: escrowPda,
        escrowVault: vaultPda,
        feeTreasury,
        mint,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .signers([recipient])
      .rpc();

    const recipientAfter = (
      await provider.connection.getTokenAccountBalance(recipientTokenAccount)
    ).value.amount;
    const treasuryAfter = (
      await provider.connection.getTokenAccountBalance(feeTreasury)
    ).value.amount;

    assert.equal(
      Number(recipientAfter),
      Number(recipientBefore) + (transferAmount - feeAmount)
    );
    assert.equal(Number(treasuryAfter), Number(treasuryBefore) + feeAmount);

    const vaultAccount = await provider.connection.getAccountInfo(vaultPda);
    assert.strictEqual(vaultAccount, null, "Escrow vault should be closed");

    const escrow = await program.account.escrowAccount.fetch(escrowPda);
    assert.deepEqual(escrow.status, { claimed: {} });
  });

  it("Fails to claim twice", async () => {
    const nonce = new anchor.BN(Date.now() + 200);
    const [escrowPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-escrow"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );
    const [vaultPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-vault"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );

    await mintTo(
      provider.connection,
      sender,
      mint,
      senderTokenAccount,
      sender,
      transferAmount
    );
    await program.methods
      .initiateTransfer(new anchor.BN(transferAmount), nonce)
      .accounts({
        sender: sender.publicKey,
        senderTokenAccount,
        recipient: recipient.publicKey,
        feeTreasury,
        mint,
        escrowAccount: escrowPda,
        escrowVault: vaultPda,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .signers([sender])
      .rpc();

    await program.methods
      .claimTransfer(nonce)
      .accounts({
        recipient: recipient.publicKey,
        recipientTokenAccount,
        sender: sender.publicKey,
        escrowAccount: escrowPda,
        escrowVault: vaultPda,
        feeTreasury,
        mint,
        tokenProgram: TOKEN_PROGRAM_ID,
      })
      .signers([recipient])
      .rpc();

    try {
      await program.methods
        .claimTransfer(nonce)
        .accounts({
          recipient: recipient.publicKey,
          recipientTokenAccount,
          sender: sender.publicKey,
          escrowAccount: escrowPda,
          escrowVault: vaultPda,
          feeTreasury,
          mint,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .signers([recipient])
        .rpc();
      assert.fail("Should have failed");
    } catch (err) {
      assert.ok(err.message, "Transfer not in a pending state");
    }
  });

  it("Fails to refund before expiry", async () => {
    const nonce = new anchor.BN(Date.now() + 300);
    const [escrowPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-escrow"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );
    const [vaultPda] = PublicKey.findProgramAddressSync(
      [
        Buffer.from("diaspora-vault"),
        sender.publicKey.toBuffer(),
        recipient.publicKey.toBuffer(),
        nonce.toBuffer("le", 8),
      ],
      program.programId
    );

    await mintTo(
      provider.connection,
      sender,
      mint,
      senderTokenAccount,
      sender,
      transferAmount
    );
    await program.methods
      .initiateTransfer(new anchor.BN(transferAmount), nonce)
      .accounts({
        sender: sender.publicKey,
        senderTokenAccount,
        recipient: recipient.publicKey,
        feeTreasury,
        mint,
        escrowAccount: escrowPda,
        escrowVault: vaultPda,
        tokenProgram: TOKEN_PROGRAM_ID,
        systemProgram: anchor.web3.SystemProgram.programId,
        rent: anchor.web3.SYSVAR_RENT_PUBKEY,
      })
      .signers([sender])
      .rpc();

    try {
      await program.methods
        .refundTransfer(nonce)
        .accounts({
          sender: sender.publicKey,
          senderTokenAccount,
          recipient: recipient.publicKey,
          escrowAccount: escrowPda,
          escrowVault: vaultPda,
          mint,
          tokenProgram: TOKEN_PROGRAM_ID,
        })
        .signers([sender])
        .rpc();
      assert.fail("Should have failed");
    } catch (err) {
      assert.ok(err.message, "Refund is not available yet");
    }
  });
});
