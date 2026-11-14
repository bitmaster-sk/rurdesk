// license-sign issues a customer license key. It is an internal tool and is not
// built into any image. The private key comes on stdin as an unencrypted PKCS#8
// PEM, so the decrypted key never has to touch the disk:
//
//	openssl pkey -in rurdesk-license-signing.pem \
//	  | go run ./commercial/cmd/license-sign --tenant acme --expires 2027-12-31
//
// The license key goes to stdout. The public key it verifies against goes to
// stderr, to be compared with the one embedded in the release.
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/licensekey"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, privateKeyPEM io.Reader, stdout, stderr io.Writer, now time.Time) error {
	flags := flag.NewFlagSet("license-sign", flag.ContinueOnError)
	flags.SetOutput(stderr)
	tenant := flags.String("tenant", "", "customer the license is issued to (required)")
	expires := flags.String("expires", "", "expiry: YYYY-MM-DD (end of that day, UTC) or RFC 3339 (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *tenant == "" {
		return errors.New("--tenant is required")
	}

	expiresAt, err := parseExpiry(*expires)
	if err != nil {
		return err
	}
	if !expiresAt.After(now) {
		return fmt.Errorf("expiry %s is not in the future", expiresAt.Format(time.RFC3339))
	}

	signingKey, err := readSigningKey(privateKeyPEM)
	if err != nil {
		return err
	}

	key, err := licensekey.Issue(signingKey, *tenant, expiresAt)
	if err != nil {
		return fmt.Errorf("issuing license key: %w", err)
	}
	publicKey := signingKey.Public().(ed25519.PublicKey)
	if _, err := licensekey.New(publicKey, key); err != nil {
		return fmt.Errorf("verifying the issued key: %w", err)
	}

	fmt.Fprintf(stderr, "tenant:     %s\nexpires at: %s\npublic key: %s\n",
		*tenant, expiresAt.Format(time.RFC3339), base64.StdEncoding.EncodeToString(publicKey))
	fmt.Fprintln(stdout, key)
	return nil
}

func parseExpiry(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("--expires is required")
	}
	if date, err := time.Parse(time.DateOnly, value); err == nil {
		return date.Add(24*time.Hour - time.Second), nil
	}
	expiresAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("--expires must be YYYY-MM-DD or RFC 3339, got %q", value)
	}
	return expiresAt.UTC(), nil
}

func readSigningKey(privateKeyPEM io.Reader) (ed25519.PrivateKey, error) {
	encoded, err := io.ReadAll(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("reading the private key from stdin: %w", err)
	}

	block, _ := pem.Decode(encoded)
	if block == nil {
		return nil, errors.New("stdin holds no PEM private key; pipe it through `openssl pkey` to decrypt it")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing the private key (is it still encrypted?): %w", err)
	}
	signingKey, isEd25519 := parsed.(ed25519.PrivateKey)
	if !isEd25519 {
		return nil, fmt.Errorf("private key is %T, not Ed25519", parsed)
	}
	return signingKey, nil
}
