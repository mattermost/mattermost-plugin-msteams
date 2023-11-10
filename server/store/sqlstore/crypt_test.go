package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	for _, test := range []struct {
		Name          string
		Key           []byte
		ExpectedError string
	}{
		{
			Name:          "EncryptDecrypt: Invalid key",
			Key:           make([]byte, 1),
			ExpectedError: "could not create a cipher block",
		},
		{
			Name: "EncryptDecrypt: Valid",
			Key:  make([]byte, 16),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			encryptedKey, err := encrypt(test.Key, "mockData")
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal("", encryptedKey)
			} else {
				assert.Nil(err)
				assert.NotEqual("", encryptedKey)
			}

			decryptedKey, err := decrypt(test.Key, encryptedKey)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal("", decryptedKey)
			} else {
				assert.Nil(err)
				assert.Equal("mockData", decryptedKey)
			}
		})
	}
}

func TestEncrypt(t *testing.T) {
	for _, test := range []struct {
		Name          string
		Key           []byte
		ExpectedError string
	}{
		{
			Name:          "Encrypt: Invalid key",
			Key:           make([]byte, 1),
			ExpectedError: "could not create a cipher block",
		},
		{
			Name: "Encrypt: Valid",
			Key:  make([]byte, 16),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			resp, err := encrypt(test.Key, "mockData")
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal("", resp)
			} else {
				assert.Nil(err)
				assert.NotEqual("", resp)
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	for _, test := range []struct {
		Name          string
		Key           []byte
		Text          string
		ExpectedError string
	}{
		{
			Name:          "Decrypt: Invalid key",
			Text:          "8qhtxbdZSjFi4-YBVmJ8nWgW2iQEoLrt8sVRTsTxm3awzvG-",
			Key:           make([]byte, 1),
			ExpectedError: "could not create a cipher block",
		},
		{
			Name: "Decrypt: Valid",
			Text: "8qhtxbdZSjFi4-YBVmJ8nWgW2iQEoLrt8sVRTsTxm3awzvG-",
			Key:  make([]byte, 16),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			resp, err := decrypt(test.Key, test.Text)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal("", resp)
			} else {
				assert.Nil(err)
				assert.Equal("mockData", resp)
			}
		})
	}
}
