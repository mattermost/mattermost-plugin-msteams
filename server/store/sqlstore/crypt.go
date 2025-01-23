// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"

	"github.com/pkg/errors"
)

func encode(encrypted []byte) string {
	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(encrypted)))
	base64.URLEncoding.Encode(encoded, encrypted)
	return string(encoded)
}

func decode(encoded string) ([]byte, error) {
	decoded := make([]byte, base64.URLEncoding.DecodedLen(len(encoded)))
	n, err := base64.URLEncoding.Decode(decoded, []byte(encoded))
	if err != nil {
		return nil, err
	}
	return decoded[:n], nil
}

func encrypt(key []byte, text string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.Wrap(err, "could not create a cipher block, check key")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := aesgcm.Seal(nil, nonce, []byte(text), nil)
	return encode(append(nonce, sealed...)), nil
}

func decrypt(key []byte, text string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.Wrap(err, "could not create a cipher block, check key")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	decoded, err := decode(text)
	if err != nil {
		return "", err
	}

	nonceSize := aesgcm.NonceSize()
	if len(decoded) < nonceSize {
		return "", errors.New("token too short")
	}

	nonce, encrypted := decoded[:nonceSize], decoded[nonceSize:]
	plain, err := aesgcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}
