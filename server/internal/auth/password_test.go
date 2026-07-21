package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("expected correct password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail verification")
	}
}

func TestHashPassword_UniqueSaltPerCall(t *testing.T) {
	h1, _ := HashPassword("same-input")
	h2, _ := HashPassword("same-input")
	if h1 == h2 {
		t.Fatal("expected two hashes of the same password to differ (random salt)")
	}
}

func TestVerifyPassword_RejectsMalformedHash(t *testing.T) {
	if VerifyPassword("not-a-real-hash", "anything") {
		t.Fatal("expected malformed hash to fail verification, not panic or match")
	}
}
