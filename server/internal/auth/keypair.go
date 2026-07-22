package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// KeyPairPEM is this platform instance's RSA keypair. PublicPEM is safe to
// hand out freely (see api's GET /api/v1/auth/public-key); PrivateKey never
// leaves the process.
//
// This pair exists for exactly one purpose: letting a self-registration
// provisioning tool (docs §4.1) tell the platform a device's real,
// factory-set secret without that secret ever crossing the network in the
// clear — the tool encrypts it against PublicPEM before sending it to
// POST /api/v1/auth/bind-key, and only this process's PrivateKey can
// recover it (see DecryptDeviceSecret). It has nothing to do with the
// device-facing HMAC scheme (internal/auth's Sign/Verify) — that remains
// entirely symmetric and per-device.
type KeyPairPEM struct {
	PrivateKey *rsa.PrivateKey
	PublicPEM  string
}

// LoadOrCreateServerKeyPair loads a 2048-bit RSA keypair from
// <dir>/server_rsa_private.pem, generating and persisting a fresh one on
// first run if that file doesn't exist yet. Loading the same file on every
// subsequent startup is what lets already-bound self-registered devices'
// secrets keep decrypting the same way a database survives a restart.
func LoadOrCreateServerKeyPair(dir string) (*KeyPairPEM, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create key dir: %w", err)
	}
	privPath := filepath.Join(dir, "server_rsa_private.pem")

	data, err := os.ReadFile(privPath)
	switch {
	case err == nil:
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid PEM in %s", privPath)
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key from %s: %w", privPath, err)
		}
		return newKeyPairPEM(key), nil
	case os.IsNotExist(err):
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate RSA key: %w", err)
		}
		privPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		// 0o600: this file is the only thing standing between "a
		// provisioning tool's encrypted submission" and "anyone who can
		// read this directory" -- unlike a device secret, it protects
		// every self-registered device's key-binding at once.
		if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
			return nil, fmt.Errorf("write %s: %w", privPath, err)
		}
		return newKeyPairPEM(key), nil
	default:
		return nil, fmt.Errorf("read %s: %w", privPath, err)
	}
}

func newKeyPairPEM(key *rsa.PrivateKey) *KeyPairPEM {
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	})
	return &KeyPairPEM{PrivateKey: key, PublicPEM: string(pubPEM)}
}

// DecryptDeviceSecret recovers a device's real secret from the
// base64-encoded RSA-OAEP-SHA256 ciphertext a provisioning tool submits to
// POST /api/v1/auth/bind-key (docs §4.1) — the inverse of that tool
// encrypting the secret it just read off the hardware (serial/BLE) against
// this platform's public key.
func DecryptDeviceSecret(priv *rsa.PrivateKey, b64Ciphertext string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(b64Ciphertext)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}
