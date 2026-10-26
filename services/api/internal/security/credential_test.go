package security

import "testing"

func TestCredentialRoundTrip(t *testing.T) {
	hash, err := HashCredential("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashCredential() error = %v", err)
	}
	ok, err := VerifyCredential(hash, "correct horse battery staple")
	if err != nil {
		t.Fatalf("VerifyCredential() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifyCredential() = false, want true")
	}
	ok, err = VerifyCredential(hash, "wrong")
	if err != nil {
		t.Fatalf("VerifyCredential(wrong) error = %v", err)
	}
	if ok {
		t.Fatal("VerifyCredential(wrong) = true, want false")
	}
}
