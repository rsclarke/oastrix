// Package token provides OAST token generation.
package token

import (
	"crypto/rand"
)

const tokenLength = 12

var charset = []byte("abcdefghijklmnopqrstuvwxyz0123456789")

func Generate() (string, error) {
	b := make([]byte, tokenLength)
	randomBytes := make([]byte, tokenLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(randomBytes[i])%len(charset)]
	}
	return string(b), nil
}
