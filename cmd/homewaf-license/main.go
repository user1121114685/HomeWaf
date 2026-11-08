// Offline signing utility. Private keys must never be included in HomeWaf releases.
package main

import (
	"HomeWaf/internal/licensing"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: homewaf-license keygen|sign|sign-plugin [options]")
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	keyPath := fs.String("key", "", "private key PEM path (outside the repository)")
	keyID := fs.String("key-id", "", "release signing key identifier")
	out := fs.String("out", "", "output path; must not exist")
	binaryPath := fs.String("binary", "", "plugin binary to sign")
	claimsPath := fs.String("claims", "", "signed-claims JSON path")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *keyPath == "" || *keyID == "" || *out == "" {
		return errors.New("--key, --key-id and --out are required")
	}
	switch os.Args[1] {
	case "keygen":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		encoded := map[string]string{*keyID: base64.RawURLEncoding.EncodeToString(pub)}
		publicJSON, _ := json.MarshalIndent(encoded, "", "  ")
		if _, err = licensing.ParseKeyring(string(publicJSON)); err != nil {
			return err
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return err
		}
		if err = writeExclusive(*keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
			return err
		}
		return writeExclusive(*out, append(publicJSON, '\n'), 0644)
	case "sign", "sign-plugin":
		if os.Args[1] == "sign" && *claimsPath == "" {
			return errors.New("--claims is required")
		}
		keyPEM, err := os.ReadFile(*keyPath)
		if err != nil {
			return err
		}
		block, _ := pem.Decode(keyPEM)
		if block == nil {
			return errors.New("invalid key PEM")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return err
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok {
			return errors.New("Ed25519 private key required")
		}
		if os.Args[1] == "sign-plugin" {
			if *binaryPath == "" {
				return errors.New("--binary is required")
			}
			file, err := os.Open(*binaryPath)
			if err != nil {
				return err
			}
			defer file.Close()
			hash := sha256.New()
			if _, err = io.Copy(hash, file); err != nil {
				return err
			}
			doc, err := licensing.SignPlugin(hex.EncodeToString(hash.Sum(nil)), *keyID, key)
			if err != nil {
				return err
			}
			return writeExclusive(*out, append(doc, '\n'), 0644)
		}
		data, err := os.ReadFile(*claimsPath)
		if err != nil {
			return err
		}
		var claims licensing.Claims
		if err = json.Unmarshal(data, &claims); err != nil {
			return err
		}
		doc, err := licensing.Sign(claims, *keyID, key)
		if err != nil {
			return err
		}
		return writeExclusive(*out, append(doc, '\n'), 0600)
	default:
		return errors.New("unknown command")
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
