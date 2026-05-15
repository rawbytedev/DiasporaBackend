package kyc

import "testing"

func TestNewKYC(t *testing.T) {
	client := NewKYC("https://api.testnet.kyc")
	if client == nil {
		t.Fatal("expected non-nil KYC client")
	}
	if client.APiurl != "https://api.testnet.kyc" {
		t.Fatalf("expected API url %q, got %q", "https://api.testnet.kyc", client.APiurl)
	}
}

func TestVerifyAndAttachInfoDoNotPanic(t *testing.T) {
	client := NewKYC("https://api.testnet.kyc")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic, got %v", r)
		}
	}()

	client.Verify()
	client.AttachInfo()
}
