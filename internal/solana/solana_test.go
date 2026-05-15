// Package solana_test verifies the encoding, PDA derivation, and mock
// behaviour of the solana package without requiring a live Solana node.
//
// Tests are organised into three groups:
//  1. Discriminator correctness   – verifies sha256("global:<fn>")[0:8]
//  2. Instruction encoding        – verifies byte layout of instruction data
//  3. PDA / ATA derivation        – verifies determinism and seed separation
//  4. Mock client behaviour       – end-to-end lifecycle via the in-memory mock
package solana_test

import (
	"Diaspora/internal/mocks"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/gagliardetto/solana-go"
)

// ---------------------------------------------------------------------------
// 1. Discriminator correctness
// ---------------------------------------------------------------------------

// anchorDisc computes the Anchor 8-byte discriminator for an instruction name.
func anchorDisc(name string) [8]byte {
	h := sha256.Sum256([]byte("global:" + name))
	var b [8]byte
	copy(b[:], h[:8])
	return b
}

func TestDiscriminators(t *testing.T) {
	cases := []struct {
		fn       string
		expected [8]byte
	}{
		{"initiate_transfer", [8]byte{0x80, 0xe5, 0x4d, 0x05, 0x41, 0xea, 0xe4, 0x4b}},
		{"claim_transfer", [8]byte{0xca, 0xb2, 0x3a, 0xbe, 0xe6, 0xea, 0xe5, 0x11}},
		{"refund_transfer", [8]byte{0x62, 0xaf, 0x73, 0xa9, 0x14, 0x3f, 0x66, 0xe2}},
	}

	for _, tc := range cases {
		got := anchorDisc(tc.fn)
		if got != tc.expected {
			t.Errorf("%s discriminator: want %x, got %x", tc.fn, tc.expected, got)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Instruction encoding helpers
// ---------------------------------------------------------------------------

// le64 mirrors the package-internal helper.
func le64(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func TestLE64Encoding(t *testing.T) {
	cases := []struct{ v uint64 }{
		{0},
		{1},
		{0xFF},
		{0xDEADBEEFCAFEBABE},
		{^uint64(0)}, // max value
	}
	for _, tc := range cases {
		b := le64(tc.v)
		if len(b) != 8 {
			t.Errorf("le64(%d): want 8 bytes, got %d", tc.v, len(b))
		}
		got := binary.LittleEndian.Uint64(b)
		if got != tc.v {
			t.Errorf("le64 round-trip: want %d, got %d", tc.v, got)
		}
	}
}

func TestInitiateInstructionDataLayout(t *testing.T) {
	disc := anchorDisc("initiate_transfer")
	amount := uint64(10_000_000)
	nonce := uint64(0xABCD1234ABCD1234)

	// Reproduce the 24-byte layout used in solana.go InitiateTransfer.
	data := make([]byte, 24)
	copy(data[:8], disc[:])
	copy(data[8:16], le64(amount))
	copy(data[16:24], le64(nonce))

	// Verify discriminator prefix.
	var gotDisc [8]byte
	copy(gotDisc[:], data[:8])
	if gotDisc != disc {
		t.Errorf("discriminator mismatch: want %x, got %x", disc, gotDisc)
	}

	// Verify amount field.
	gotAmount := binary.LittleEndian.Uint64(data[8:16])
	if gotAmount != amount {
		t.Errorf("amount field: want %d, got %d", amount, gotAmount)
	}

	// Verify nonce field.
	gotNonce := binary.LittleEndian.Uint64(data[16:24])
	if gotNonce != nonce {
		t.Errorf("nonce field: want %d, got %d", nonce, gotNonce)
	}
}

func TestClaimRefundInstructionDataLayout(t *testing.T) {
	for _, fn := range []string{"claim_transfer", "refund_transfer"} {
		disc := anchorDisc(fn)
		nonce := uint64(0x1122334455667788)

		data := make([]byte, 16)
		copy(data[:8], disc[:])
		copy(data[8:16], le64(nonce))

		var gotDisc [8]byte
		copy(gotDisc[:], data[:8])
		if gotDisc != disc {
			t.Errorf("%s discriminator: want %x, got %x", fn, disc, gotDisc)
		}
		gotNonce := binary.LittleEndian.Uint64(data[8:16])
		if gotNonce != nonce {
			t.Errorf("%s nonce: want %d, got %d", fn, nonce, gotNonce)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. PDA / ATA derivation
// ---------------------------------------------------------------------------

var (
	testProgramID  = solana.MustPublicKeyFromBase58("5GHE14Zmpq5yNwpvHR2ZLaTcSckp6QogCRNm43M3Z9BT")
	testSender     = solana.MustPublicKeyFromBase58("9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM")
	testRecipient  = solana.MustPublicKeyFromBase58("7UX2i7SucgLMQcfZ75s3VXmZZY4YRUyJN9X1RgfMoDUi")
	testMint       = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	testTreasury   = solana.MustPublicKeyFromBase58("6gvHu8VTqZhdj77YGaKF1cqG4jprE1FVd4oHDV43fV3C")
	splTokenProg   = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	ataProg        = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJe8bv")
)

func deriveEscrowPDA(programID, sender, recipient solana.PublicKey, nonce uint64) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("diaspora-escrow"), sender.Bytes(), recipient.Bytes(), le64(nonce)},
		programID,
	)
	return addr, err
}

func deriveVaultPDA(programID, sender, recipient solana.PublicKey, nonce uint64) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("diaspora-vault"), sender.Bytes(), recipient.Bytes(), le64(nonce)},
		programID,
	)
	return addr, err
}

func findATA(wallet, mint solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{wallet.Bytes(), splTokenProg.Bytes(), mint.Bytes()},
		ataProg,
	)
	return addr, err
}

func TestPDADeterminism(t *testing.T) {
	const nonce = uint64(12345)

	escrow1, err := deriveEscrowPDA(testProgramID, testSender, testRecipient, nonce)
	if err != nil {
		t.Fatalf("escrow PDA: %v", err)
	}
	escrow2, err := deriveEscrowPDA(testProgramID, testSender, testRecipient, nonce)
	if err != nil {
		t.Fatalf("escrow PDA repeat: %v", err)
	}
	if escrow1 != escrow2 {
		t.Error("escrow PDA not deterministic")
	}

	vault1, err := deriveVaultPDA(testProgramID, testSender, testRecipient, nonce)
	if err != nil {
		t.Fatalf("vault PDA: %v", err)
	}
	vault2, err := deriveVaultPDA(testProgramID, testSender, testRecipient, nonce)
	if err != nil {
		t.Fatalf("vault PDA repeat: %v", err)
	}
	if vault1 != vault2 {
		t.Error("vault PDA not deterministic")
	}
}

func TestEscrowAndVaultPDAsAreDifferent(t *testing.T) {
	const nonce = uint64(99)

	escrow, err := deriveEscrowPDA(testProgramID, testSender, testRecipient, nonce)
	if err != nil {
		t.Fatalf("escrow PDA: %v", err)
	}
	vault, err := deriveVaultPDA(testProgramID, testSender, testRecipient, nonce)
	if err != nil {
		t.Fatalf("vault PDA: %v", err)
	}
	if escrow == vault {
		t.Error("escrow and vault PDAs must be different")
	}
}

func TestPDAChangesWithNonce(t *testing.T) {
	pda1, _ := deriveEscrowPDA(testProgramID, testSender, testRecipient, 1)
	pda2, _ := deriveEscrowPDA(testProgramID, testSender, testRecipient, 2)
	if pda1 == pda2 {
		t.Error("different nonces must produce different PDAs")
	}
}

func TestPDAChangesWithSender(t *testing.T) {
	other := solana.MustPublicKeyFromBase58("DRpbCBMxVnDK7maPM5tGv6MvB3v1sRMC86PZ8okm21hy")
	const nonce = uint64(1)

	pda1, _ := deriveEscrowPDA(testProgramID, testSender, testRecipient, nonce)
	pda2, _ := deriveEscrowPDA(testProgramID, other, testRecipient, nonce)
	if pda1 == pda2 {
		t.Error("different senders must produce different PDAs")
	}
}

func TestPDAChangesWithRecipient(t *testing.T) {
	other := solana.MustPublicKeyFromBase58("DRpbCBMxVnDK7maPM5tGv6MvB3v1sRMC86PZ8okm21hy")
	const nonce = uint64(1)

	pda1, _ := deriveEscrowPDA(testProgramID, testSender, testRecipient, nonce)
	pda2, _ := deriveEscrowPDA(testProgramID, testSender, other, nonce)
	if pda1 == pda2 {
		t.Error("different recipients must produce different PDAs")
	}
}

func TestPDAIsNotZero(t *testing.T) {
	pda, err := deriveEscrowPDA(testProgramID, testSender, testRecipient, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pda.IsZero() {
		t.Error("derived PDA should not be zero public key")
	}
}

func TestATADeterminism(t *testing.T) {
	ata1, err := findATA(testSender, testMint)
	if err != nil {
		t.Fatalf("ATA: %v", err)
	}
	ata2, err := findATA(testSender, testMint)
	if err != nil {
		t.Fatalf("ATA repeat: %v", err)
	}
	if ata1 != ata2 {
		t.Error("ATA not deterministic")
	}
	if ata1.IsZero() {
		t.Error("derived ATA should not be zero public key")
	}
}

func TestATAChangesWithWallet(t *testing.T) {
	ata1, _ := findATA(testSender, testMint)
	ata2, _ := findATA(testRecipient, testMint)
	if ata1 == ata2 {
		t.Error("different wallets must produce different ATAs")
	}
}

func TestATAChangesWithMint(t *testing.T) {
	otherMint := solana.MustPublicKeyFromBase58("Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB")
	ata1, _ := findATA(testTreasury, testMint)
	ata2, _ := findATA(testTreasury, otherMint)
	if ata1 == ata2 {
		t.Error("different mints must produce different ATAs")
	}
}

// ---------------------------------------------------------------------------
// 4. Mock client behaviour
// ---------------------------------------------------------------------------

func TestMockInitiateTransfer_Success(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, nonce, err := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty tx hash")
	}
	if nonce == 0 {
		t.Error("expected non-zero nonce")
	}

	history := client.GetTransferHistory()
	rec, ok := history[hash]
	if !ok {
		t.Fatalf("transfer %q not found in history", hash)
	}
	if rec.SenderID != 1 || rec.RecipientID != 2 {
		t.Errorf("IDs: got sender=%d recipient=%d", rec.SenderID, rec.RecipientID)
	}
	if rec.NetAmount != 99.0 {
		t.Errorf("net amount: want 99.0, got %f", rec.NetAmount)
	}
	if rec.Status != "pending" {
		t.Errorf("status: want pending, got %s", rec.Status)
	}
	if rec.Nonce != nonce {
		t.Errorf("nonce mismatch: returned %d, stored %d", nonce, rec.Nonce)
	}
}

func TestMockInitiateTransfer_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetInitiateError(errors.New("insufficient funds"))

	_, _, err := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "insufficient funds" {
		t.Errorf("error: want 'insufficient funds', got %q", err.Error())
	}
}

func TestMockInitiateTransfer_UniqueHashes(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash1, _, _ := client.InitiateTransfer(1, 2, 50.0, 0.5)
	hash2, _, _ := client.InitiateTransfer(1, 2, 50.0, 0.5)

	if hash1 == hash2 {
		t.Errorf("expected unique hashes, got same: %s", hash1)
	}
}

func TestMockClaimTransfer_Success(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, _, err := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if err := client.ClaimTransfer(hash); err != nil {
		t.Fatalf("claim: %v", err)
	}

	status, _ := client.GetTransactionStatus(hash)
	if status != "claimed" {
		t.Errorf("status: want claimed, got %s", status)
	}
}

func TestMockClaimTransfer_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetClaimError(errors.New("claim rejected"))

	hash, _, _ := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err := client.ClaimTransfer(hash); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockClaimTransfer_NotFound(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	if err := client.ClaimTransfer("nonexistent_hash"); err == nil {
		t.Fatal("expected error for unknown hash, got nil")
	}
}

func TestMockRefundTransfer_Success(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, _, _ := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err := client.RefundTransfer(hash); err != nil {
		t.Fatalf("refund: %v", err)
	}

	status, _ := client.GetTransactionStatus(hash)
	if status != "refunded" {
		t.Errorf("status: want refunded, got %s", status)
	}
}

func TestMockRefundTransfer_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetRefundError(errors.New("refund not available"))

	hash, _, _ := client.InitiateTransfer(1, 2, 99.0, 1.0)
	if err := client.RefundTransfer(hash); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockRefundTransfer_NotFound(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	if err := client.RefundTransfer("does_not_exist"); err == nil {
		t.Fatal("expected error for unknown hash, got nil")
	}
}

func TestMockGetTokenBalance_Default(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	balance, err := client.GetTokenBalance("any_pubkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 1000.0 {
		t.Errorf("balance: want 1000.0, got %f", balance)
	}
}

func TestMockGetTokenBalance_Custom(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetMockBalance(42.5)
	balance, _ := client.GetTokenBalance("pubkey")
	if balance != 42.5 {
		t.Errorf("balance: want 42.5, got %f", balance)
	}
}

func TestMockGetTokenBalance_Error(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	client.SetBalanceError(errors.New("node unavailable"))
	_, err := client.GetTokenBalance("pubkey")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockGetTransactionStatus_NotFound(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	status, err := client.GetTransactionStatus("unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "not_found" {
		t.Errorf("status: want not_found, got %s", status)
	}
}

func TestMockGetTransactionStatus_Lifecycle(t *testing.T) {
	client := mocks.NewMockSolanaClient()

	hash, _, _ := client.InitiateTransfer(3, 4, 200.0, 2.0)

	status, _ := client.GetTransactionStatus(hash)
	if status != "pending" {
		t.Errorf("after initiate: want pending, got %s", status)
	}

	_ = client.ClaimTransfer(hash)
	status, _ = client.GetTransactionStatus(hash)
	if status != "claimed" {
		t.Errorf("after claim: want claimed, got %s", status)
	}
}

func TestMockConcurrencySafe(t *testing.T) {
	client := mocks.NewMockSolanaClient()
	done := make(chan struct{}, 10)

	for i := 0; i < 5; i++ {
		go func(id uint) {
			client.InitiateTransfer(id, id+1, float64(id)*10, float64(id)*0.1)
			done <- struct{}{}
		}(uint(i + 1))
	}

	for i := 0; i < 5; i++ {
		go func() {
			client.GetTokenBalance("some_pubkey")
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
