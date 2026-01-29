package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"math/big"
	"strings"
)

const (
	servicePrefix = "oastrix"
	prefixLength  = 12
	secretBytes   = 32
)

var ErrInvalidKeyFormat = errors.New("invalid API key format")

func GenerateAPIKey() (displayKey string, prefix string, hash []byte, err error) {
	prefixBytes := make([]byte, prefixLength)
	if _, err := rand.Read(prefixBytes); err != nil {
		return "", "", nil, err
	}
	for i := range prefixBytes {
		prefixBytes[i] = alphanumeric[int(prefixBytes[i])%len(alphanumeric)]
	}
	prefix = string(prefixBytes)

	secretRaw := make([]byte, secretBytes)
	if _, err := rand.Read(secretRaw); err != nil {
		return "", "", nil, err
	}
	secret := encodeBase62(secretRaw)

	displayKey = servicePrefix + "_" + prefix + "_" + secret
	hash = HashSecret(secret)

	return displayKey, prefix, hash, nil
}

func HashSecret(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func VerifyAPIKey(displayKey string, storedHash []byte) bool {
	prefix, secret, err := ParseAPIKey(displayKey)
	if err != nil || prefix == "" {
		return false
	}
	computedHash := HashSecret(secret)
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

func ParseAPIKey(displayKey string) (prefix string, secret string, err error) {
	// Format: oastrix_<prefix>_<secret>
	if !strings.HasPrefix(displayKey, servicePrefix+"_") {
		return "", "", ErrInvalidKeyFormat
	}
	rest := strings.TrimPrefix(displayKey, servicePrefix+"_")
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidKeyFormat
	}
	if len(parts[0]) != prefixLength {
		return "", "", ErrInvalidKeyFormat
	}
	for _, c := range parts[0] {
		if !isAlphanumeric(c) {
			return "", "", ErrInvalidKeyFormat
		}
	}
	return parts[0], parts[1], nil
}

var alphanumeric = []byte("abcdefghijklmnopqrstuvwxyz0123456789")

// base62Alphabet includes A-Za-z0-9 (no special characters)
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func encodeBase62(data []byte) string {
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	var result []byte

	for num.Cmp(zero) > 0 {
		mod := new(big.Int)
		num.DivMod(num, base, mod)
		result = append([]byte{base62Alphabet[mod.Int64()]}, result...)
	}

	// Preserve leading zeros
	for _, b := range data {
		if b != 0 {
			break
		}
		result = append([]byte{'0'}, result...)
	}

	if len(result) == 0 {
		return "0"
	}
	return string(result)
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
