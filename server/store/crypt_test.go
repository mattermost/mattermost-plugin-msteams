package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPad(t *testing.T) {
	assert := assert.New(t)
	resp := pad(make([]byte, 3))
	assert.Equal([]byte{0x0, 0x0, 0x0, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd, 0xd}, resp)
}

func TestUnpad(t *testing.T) {
	assert := assert.New(t)
	resp, err := unpad(make([]byte, 1))
	assert.Nil(err)
	assert.Equal([]byte{0x0}, resp)
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
			Text:          "g2E9QKddQTJn74EFipHhZ7QCW0vf1cIzXzN8xRTr9fA=",
			Key:           make([]byte, 1),
			ExpectedError: "could not create a cipher block",
		},
		{
			Name:          "Decrypt: blocksize must be multiple of decoded message length",
			Text:          "mockData",
			Key:           make([]byte, 16),
			ExpectedError: "blocksize must be multiple of decoded message length",
		},
		{
			Name: "Decrypt: Valid",
			Text: "g2E9QKddQTJn74EFipHhZ7QCW0vf1cIzXzN8xRTr9fA=",
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
