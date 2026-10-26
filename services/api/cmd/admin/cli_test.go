package main

import (
	"strings"
	"testing"
)

func TestParseCreateArgs(t *testing.T) {
	options, err := parseCreateArgs([]string{"--email", "admin@example.com", "--credential-stdin"})
	if err != nil {
		t.Fatalf("parseCreateArgs() error = %v", err)
	}
	if options.Email != "admin@example.com" {
		t.Fatalf("Email = %q", options.Email)
	}
	if options.Role != "ADMIN" {
		t.Fatalf("Role = %q", options.Role)
	}
	if !options.CredentialStdin {
		t.Fatal("CredentialStdin = false")
	}
}

func TestParseCreateArgsRejectsMissingEmail(t *testing.T) {
	_, err := parseCreateArgs([]string{"--credential-stdin"})
	if err == nil {
		t.Fatal("expected missing email to fail")
	}
}

func TestParseCreateArgsRejectsUnsupportedRole(t *testing.T) {
	_, err := parseCreateArgs([]string{
		"--email", "admin@example.com",
		"--role", "OWNER",
		"--credential-stdin",
	})
	if err == nil {
		t.Fatal("expected unsupported role to fail")
	}
}

func TestReadCredential(t *testing.T) {
	credential, err := readCredential(strings.NewReader("dev-credential-1234\n"))
	if err != nil {
		t.Fatalf("readCredential() error = %v", err)
	}
	if credential != "dev-credential-1234" {
		t.Fatalf("credential = %q", credential)
	}
}

func TestReadCredentialRejectsEmptyValue(t *testing.T) {
	_, err := readCredential(strings.NewReader("   \n"))
	if err == nil {
		t.Fatal("expected empty credential to fail")
	}
}

func TestReadCredentialRejectsShortValue(t *testing.T) {
	_, err := readCredential(strings.NewReader("short\n"))
	if err == nil {
		t.Fatal("expected short credential to fail")
	}
}
