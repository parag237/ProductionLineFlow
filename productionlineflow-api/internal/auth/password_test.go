package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if encoded == "correct horse battery staple" {
		t.Fatal("password must not be stored as plaintext")
	}
	if !VerifyPassword("correct horse battery staple", encoded) {
		t.Fatal("expected password verification to succeed")
	}
	if VerifyPassword("wrong password", encoded) {
		t.Fatal("expected wrong password verification to fail")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	if VerifyPassword("password", "not-an-argon2id-hash") {
		t.Fatal("expected malformed hash to fail")
	}
}
