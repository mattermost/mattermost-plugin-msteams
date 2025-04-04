// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	keyID           = "my-key-id"
	keyWithoutAlgID = "SjE4tvzAwAoo6GB32-g1QAdgIck"
)

// TestValidateToken was inspired by https://github.com/MicahParks/keyfunc/blob/main/keyfunc_test.go.
func TestValidateToken(t *testing.T) {
	makeKeySet := func(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, keyfunc.Keyfunc) {
		serverStore := jwkset.NewMemoryStorage()

		// Make a public/private key that has the alg property set.
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		jwk, err := jwkset.NewJWKFromKey(priv, jwkset.JWKOptions{
			Metadata: jwkset.JWKMetadataOptions{
				KID: keyID,
				USE: jwkset.UseSig,
			},
		})
		require.NoError(t, err)

		err = serverStore.KeyWrite(context.TODO(), jwk)
		require.NoError(t, err)

		// Make a public/private key that is missing the alg property.
		jwk2, err := jwkset.NewJWKFromRawJSON(
			json.RawMessage(`
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "SjE4tvzAwAoo6GB32-g1QAdgIck",
					"x5t": "SjE4tvzAwAoo6GB32-g1QAdgIck",
					"n": "ul88fCCUH0e4sqPqWOFj9BWGIctw2JJhoBO2aOykMvbjgr3Sn0ZbitaJTi5L8HFISLmwdSGvj76SOe7qNV0Jb0PuOb5DWTB_f4hXXPqZLfh5Bn7uyuTRapbaRczDESR1BuubTodJyhYapb1B19F4EbMbmvce2kXRRWZ5OFJA_FR7ZMU2mwLD5yzuWo_gr_52FwZZSBX1fkPbmDLriJoEIl8IVMMK11hlyK-m0LYsT-Tz_AHX3eT2bct-4xQSZAKsiWj68q4a6ek5LO5oM1MrkoFhErCDMWz-N8v7mM1qyy_kUQ417ZBBNGg5IvoIuM8yYQLMsH7R3i24UpT_kkJE6w",
					"e": "AQAB",
					"x5c": [
						"MIIC/TCCAeWgAwIBAgIIDlcb6PCgUSgwDQYJKoZIhvcNAQELBQAwLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDAeFw0yNDA4MDQxNjA1NTFaFw0yOTA4MDQxNjA1NTFaMC0xKzApBgNVBAMTImFjY291bnRzLmFjY2Vzc2NvbnRyb2wud2luZG93cy5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC6Xzx8IJQfR7iyo+pY4WP0FYYhy3DYkmGgE7Zo7KQy9uOCvdKfRluK1olOLkvwcUhIubB1Ia+PvpI57uo1XQlvQ+45vkNZMH9/iFdc+pkt+HkGfu7K5NFqltpFzMMRJHUG65tOh0nKFhqlvUHX0XgRsxua9x7aRdFFZnk4UkD8VHtkxTabAsPnLO5aj+Cv/nYXBllIFfV+Q9uYMuuImgQiXwhUwwrXWGXIr6bQtixP5PP8Adfd5PZty37jFBJkAqyJaPryrhrp6Tks7mgzUyuSgWESsIMxbP43y/uYzWrLL+RRDjXtkEE0aDki+gi4zzJhAsywftHeLbhSlP+SQkTrAgMBAAGjITAfMB0GA1UdDgQWBBS+wOJGOC8r3kutKW7UjRnXV2QlBjANBgkqhkiG9w0BAQsFAAOCAQEAtGOU0QsTPGFSteuIf1N9gM+qiONQqgfb66+FT/eXvuacFMa4pgXpUN0/AuKMxBg5kDRcms2PibWzefZ7RrRfLosKtViwVqkkKK+oyuSYXVArz+8u/v+jEgBh3BoMPqB3ukvCpGTB0rHX+QV1zNBac7hVQs/4kEGcr2/Nsa1g/uVRh2N7LQo9YRImmeOk/JrxgaSbkioW1xsQKMv7ZJLSLaSLXhAvA3HUU2kHMJCXE2VkNrs/naA47dWkMa9Af1GeqOe8uH+EJu88xz78kwKk2EiZt41ZaTY57fXYCxlnNQzhRdvm1KmJ8OfMUa/pqtXKWzrPWL/vs2oDsZJz9DzERw=="
					],
					"issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
				}
			`),
			jwkset.JWKMarshalOptions{
				Private: true,
			},
			jwkset.JWKValidateOptions{},
		)
		require.NoError(t, err)

		err = serverStore.KeyWrite(context.TODO(), jwk2)
		require.NoError(t, err)

		// Finally, setup the keyfunc backed by the above memory store.
		options := keyfunc.Options{
			Ctx:          context.TODO(),
			Storage:      serverStore,
			UseWhitelist: []jwkset.USE{jwkset.UseSig},
		}
		k, err := keyfunc.New(options)
		if err != nil {
			t.Fatalf("Failed to create Keyfunc. Error: %s", err)
		}

		return pub, priv, k
	}

	newToken := func(t *testing.T, priv ed25519.PrivateKey, mapClaims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, mapClaims)
		token.Header[jwkset.HeaderKID] = keyID
		signed, err := token.SignedString(priv)
		if err != nil {
			t.Fatalf("Failed to sign JWT. Error: %s", err)
		}

		return signed
	}

	past := func() int64 {
		return time.Now().Add(-60 * time.Second).Unix()
	}

	future := func() int64 {
		return time.Now().Add(60 * time.Second).Unix()
	}

	const (
		TestSiteURL  = "https://example.com"
		TestClientID = "app-id"
	)

	makeValidateTokenParams := func(jwtKeyFunc keyfunc.Keyfunc, token string, expectedTenantIDs []string, enableDeveloper bool) validateTokenParams {
		return validateTokenParams{
			jwtKeyFunc:        jwtKeyFunc,
			token:             token,
			expectedTenantIDs: expectedTenantIDs,
			enableDeveloper:   enableDeveloper,
			siteURL:           TestSiteURL,
			clientID:          TestClientID,
		}
	}

	runPermutations(t, false, func(t *testing.T, enableDeveloper bool) {
		t.Run("empty authorization header", func(t *testing.T) {
			_, _, jwtKeyFunc := makeKeySet(t)
			params := makeValidateTokenParams(jwtKeyFunc, "", []string{}, enableDeveloper)

			_, validationErr := validateToken(params)
			if enableDeveloper {
				assert.Nil(t, validationErr)
			} else {
				require.NotNil(t, validationErr)
				assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
			}
		})

		t.Run("nil keyfunc", func(t *testing.T) {
			params := makeValidateTokenParams(nil, "invalid", []string{}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusInternalServerError, validationErr.StatusCode)
		})

		t.Run("failed to parse authorization header", func(t *testing.T) {
			_, _, jwtKeyFunc := makeKeySet(t)
			params := makeValidateTokenParams(jwtKeyFunc, "invalid", []string{}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, missing claims", func(t *testing.T) {
			_, priv, jwtKeyFunc := makeKeySet(t)
			params := makeValidateTokenParams(jwtKeyFunc, newToken(t, priv, nil), []string{}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("hmac key pretending to be rsa", func(t *testing.T) {
			tid := uuid.NewString()
			pub, _, jwtKeyFunc := makeKeySet(t)

			jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": tid,
			})
			jwtToken.Header[jwkset.HeaderKID] = keyWithoutAlgID
			token, err := jwtToken.SignedString([]byte(pub))
			require.NoError(t, err)

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, missing iat claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": tid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, invalid iat claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": "invalid",
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": tid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, future iat claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": future(),
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": tid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, missing exp claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"nbf": past(),
				"tid": tid,
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, invalid exp claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": "invalid",
				"nbf": past(),
				"tid": tid,
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, expired exp claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": past(),
				"nbf": past(),
				"tid": tid,
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, missing nbf claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"tid": tid,
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, invalid nbf claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": "invalid",
				"tid": tid,
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, future nbf claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": future(),
				"tid": tid,
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, wrong aud claim", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"tid": tid,
				"aud": "unexpected-app",
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			if enableDeveloper {
				assert.Nil(t, validationErr)
			} else {
				require.NotNil(t, validationErr)
				assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
			}
		})

		t.Run("signed token, no tenants configured", func(t *testing.T) {
			wrongTid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": wrongTid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, not matching single configured tenant", func(t *testing.T) {
			wrongTid := uuid.NewString()
			expectedTid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": wrongTid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{expectedTid}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, not matching multiple configured tenants", func(t *testing.T) {
			wrongTid := uuid.NewString()
			expectedTid1 := uuid.NewString()
			expectedTid2 := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": wrongTid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{expectedTid1, expectedTid2}, enableDeveloper)

			_, validationErr := validateToken(params)
			require.NotNil(t, validationErr)
			assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
		})

		t.Run("signed token, matching single configured tenant", func(t *testing.T) {
			tid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": ExpectedAudience,
				"tid": tid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{tid}, enableDeveloper)

			_, validationErr := validateToken(params)
			assert.Nil(t, validationErr)
		})

		t.Run("signed token, matching one of multiple configured tenants", func(t *testing.T) {
			expectedTid1 := uuid.NewString()
			expectedTid2 := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": ExpectedAudience,
				"tid": expectedTid1,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{expectedTid1, expectedTid2}, enableDeveloper)

			_, validationErr := validateToken(params)
			assert.Nil(t, validationErr)
		})

		t.Run("signed token, wildcard tenant", func(t *testing.T) {
			developerTid := uuid.NewString()

			_, priv, jwtKeyFunc := makeKeySet(t)
			token := newToken(t, priv, jwt.MapClaims{
				"iat": past(),
				"exp": future(),
				"nbf": past(),
				"aud": fmt.Sprintf(ExpectedAudienceFmt, TestSiteURL, TestClientID),
				"tid": developerTid,
			})

			params := makeValidateTokenParams(jwtKeyFunc, token, []string{"*"}, enableDeveloper)

			_, validationErr := validateToken(params)
			if enableDeveloper {
				assert.Nil(t, validationErr)
			} else {
				require.NotNil(t, validationErr)
				assert.Equal(t, http.StatusUnauthorized, validationErr.StatusCode)
			}
		})
	})
}
