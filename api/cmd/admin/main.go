// admin is the maintenance CLI shipped inside the production image. Run it with
// `docker exec -it <container> admin <command>`. It reads secrets from env vars
// when present, otherwise prompts for them interactively (masked).
//
// Commands:
//
//	rotate-git-key   re-encrypt stored git integration tokens with a new key
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/adminops"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "rotate-git-key":
		if err := rotateGitKey(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `admin — RURDESK maintenance CLI

Usage:
  admin <command>

Commands:
  rotate-git-key   Re-encrypt stored git integration tokens with a new key.
                   Reads the current key from GIT_INTEGRATION_ENCRYPTION_KEY and
                   the new key from NEW_ENCRYPTION_KEY; prompts (masked) for any
                   that are unset. Database connection uses the DATABASE_* env
                   vars (same as the app).
`)
}

func rotateGitKey(ctx context.Context) error {
	oldKey, err := readKey("GIT_INTEGRATION_ENCRYPTION_KEY", "Current encryption key (base64): ")
	if err != nil {
		return err
	}
	newKey, err := readKey("NEW_ENCRYPTION_KEY", "New encryption key (base64): ")
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, dbConnStr())
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	n, err := adminops.RotateGitTokens(ctx, pool, oldKey, newKey)
	if err != nil {
		return fmt.Errorf("rotating tokens: %w", err)
	}
	fmt.Printf("rotated %d token(s)\n", n)
	return nil
}

// readKey resolves a 32-byte base64 key from an env var, falling back to a masked
// interactive prompt when the var is unset.
func readKey(envVar, prompt string) ([]byte, error) {
	raw := os.Getenv(envVar)
	if raw == "" {
		var err error
		raw, err = promptSecret(prompt)
		if err != nil {
			return nil, err
		}
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", envVar, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%s must be 32 bytes (got %d)", envVar, len(key))
	}
	return key, nil
}

// promptSecret reads a line from the terminal without echoing it.
func promptSecret(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("no value provided and stdin is not a terminal — set the env var or run with -it")
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return string(b), nil
}

func dbConnStr() string {
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		get("DATABASE_HOST", "localhost"),
		get("DATABASE_PORT", "5432"),
		get("DATABASE_NAME", "rurdesk"),
		get("DATABASE_USER", "rurdesk"),
		get("DATABASE_PASSWORD", "rurdesk"),
	)
}
