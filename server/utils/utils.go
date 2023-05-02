package utils

import (
	"crypto/rand"
	"encoding/base32"
	"math/big"
	"strings"

	"github.com/pborman/uuid"
)

func GenerateRandomPassword() string {
	lowerCharSet := "abcdedfghijklmnopqrst"
	upperCharSet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialCharSet := "!@#$%&*"
	numberSet := "0123456789"
	allCharSet := lowerCharSet + upperCharSet + specialCharSet + numberSet

	var password strings.Builder

	password.WriteString(getRandomString(lowerCharSet, 1))
	password.WriteString(getRandomString(upperCharSet, 1))
	password.WriteString(getRandomString(specialCharSet, 1))
	password.WriteString(getRandomString(numberSet, 1))
	password.WriteString(getRandomString(allCharSet, 20))
	return password.String()
}

func getRandomString(characterSet string, length int) string {
	var randomString strings.Builder
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(length)))
		randomString.WriteString(string(characterSet[num.Int64()]))
	}

	return randomString.String()
}

func IsMSTeamsUser(remoteID, username string) bool {
	data := strings.Split(username, "_")
	if len(data) >= 2 {
		msUserID := data[len(data)-1]

		userUUID := uuid.Parse(msUserID)
		encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
		shortUserID := encoding.EncodeToString(userUUID)

		return remoteID == shortUserID
	}

	return false
}
