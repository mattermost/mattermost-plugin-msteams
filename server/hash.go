package main

import (
	"crypto/sha256"
	"encoding/hex"
)

func generateHash(teamID, channelID, secret string) string {
	h := sha256.New()
	h.Write([]byte(teamID))
	h.Write([]byte(channelID))
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(nil))
}
