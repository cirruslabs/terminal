package server

import (
	"crypto/sha256"
	"encoding/hex"
	"go.uber.org/zap"
)

const (
	locatorField = "terminal-locator"
	tokenField   = "terminal-token-hashed"
	secretField  = "terminal-secret-hashed"
)

func LocatorField(locator string) zap.Field {
	return zap.String(locatorField, locator)
}

func HashedTokenField(token string) zap.Field {
	return zap.String(tokenField, hashed(token))
}

func HashedSecretField(secret string) zap.Field {
	return zap.String(secretField, hashed(secret))
}

func hashed(s string) string {
	digest := sha256.Sum256([]byte(s))
	return hex.EncodeToString(digest[:])
}
