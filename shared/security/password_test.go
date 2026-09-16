package security

import "testing"

func TestHashAndVerifyPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword() returned an empty string")
	}

	ok, err := VerifyPassword("correct-horse-battery-staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() = false for the correct password, want true")
	}
}

func TestVerifyPassword_RejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("the-real-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	ok, err := VerifyPassword("a-guess", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Error("VerifyPassword() = true for the wrong password, want false")
	}
}

// Two hashes of the same password must differ — this is what proves a
// random salt is actually being used (see the field on defaultParams).
// Without this, two staff members who happen to pick the same password
// would have identical rows in the database.
func TestHashPassword_UsesRandomSalt(t *testing.T) {
	hash1, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	hash2, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash1 == hash2 {
		t.Error("two hashes of the same password must not be identical (salt is not being randomized)")
	}
}

func TestVerifyPassword_RejectsUnrecognizedFormat(t *testing.T) {
	_, err := VerifyPassword("anything", "not-an-argon2-hash")
	if err == nil {
		t.Error("expected VerifyPassword() to reject a malformed hash string, got no error")
	}
}

func TestVerifyPassword_RejectsTamperedHash(t *testing.T) {
	hash, err := HashPassword("some-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	// Flip the last character of the encoded hash portion.
	tampered := hash[:len(hash)-1] + "x"

	ok, _ := VerifyPassword("some-password", tampered)
	if ok {
		t.Error("VerifyPassword() = true for a tampered hash, want false")
	}
}
