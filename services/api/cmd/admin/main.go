package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/believeboy/mini-bd-program/services/api/internal/database"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/store"
)

type createOptions struct {
	Email           string
	Role            string
	CredentialStdin bool
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "create" {
		fmt.Fprintln(os.Stderr, "usage: admin create --email EMAIL [--role ADMIN|STAFF] [--credential-stdin]")
		os.Exit(2)
	}
	options, err := parseCreateArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := runCreate(options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseCreateArgs(args []string) (createOptions, error) {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "administrator email")
	role := flags.String("role", "ADMIN", "ADMIN or STAFF")
	credentialStdin := flags.Bool("credential-stdin", false, "read credential from stdin")
	if err := flags.Parse(args); err != nil {
		return createOptions{}, err
	}
	if flags.NArg() != 0 {
		return createOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	if normalizedEmail == "" {
		return createOptions{}, errors.New("--email is required")
	}
	address, err := mail.ParseAddress(normalizedEmail)
	if err != nil || address.Address != normalizedEmail {
		return createOptions{}, errors.New("--email must be a valid email address")
	}
	normalizedRole := strings.ToUpper(strings.TrimSpace(*role))
	if normalizedRole != "ADMIN" && normalizedRole != "STAFF" {
		return createOptions{}, errors.New("--role must be ADMIN or STAFF")
	}
	return createOptions{
		Email:           normalizedEmail,
		Role:            normalizedRole,
		CredentialStdin: *credentialStdin,
	}, nil
}

func runCreate(options createOptions) error {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	credential, err := readCredentialInput(options.CredentialStdin)
	if err != nil {
		return err
	}
	credentialHash, err := security.HashCredential(credential)
	if err != nil {
		return fmt.Errorf("hash credential: %w", err)
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	admin, err := store.New(pool).CreateAdmin(ctx, store.CreateAdminParams{
		ID:             uuid.New(),
		Email:          options.Email,
		CredentialHash: credentialHash,
		Role:           options.Role,
		Status:         "ACTIVE",
	})
	if err != nil {
		return fmt.Errorf("create administrator: %w", err)
	}
	fmt.Fprintf(os.Stdout, "created administrator %s with role %s (%s)\n", admin.Email, admin.Role, admin.ID)
	return nil
}

func readCredentialInput(fromStdin bool) (string, error) {
	if fromStdin {
		return readCredential(os.Stdin)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("non-interactive input requires --credential-stdin")
	}
	fmt.Fprint(os.Stderr, "Credential: ")
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read credential: %w", err)
	}
	return validateCredential(string(value))
}

func readCredential(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 4096)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read credential: %w", err)
		}
		return "", errors.New("credential is required")
	}
	return validateCredential(scanner.Text())
}

func validateCredential(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 12 {
		return "", errors.New("credential must contain at least 12 characters")
	}
	return value, nil
}
