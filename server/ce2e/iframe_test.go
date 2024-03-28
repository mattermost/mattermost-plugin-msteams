//go:build exclude

package ce2e

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-msteams/assets"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils/containere2e"
)

func TestIFrame(t *testing.T) {
	mattermost, _, _, tearDown := containere2e.NewE2ETestPlugin(t)
	defer tearDown()
	client, err := mattermost.GetAdminClient(context.Background())
	require.NoError(t, err)

	url, err := mattermost.URL(context.Background())
	require.NoError(t, err)

	t.Run("iframe tab", func(t *testing.T) {
		reqURL := client.URL + "/plugins/com.mattermost.msteams-sync/iframe/mattermostTab"
		resp, err := client.DoAPIRequest(context.Background(), http.MethodGet, reqURL, "", "")
		require.NoError(t, err, "cannot fetch url %s", reqURL)
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		require.Nil(t, err)
		bodyString := string(bodyBytes)

		require.Equal(t, 200, resp.StatusCode)

		assert.Contains(t, bodyString, "<!DOCTYPE html>")

		expect := fmt.Sprintf(`src="%s" title="Mattermost"`, url)
		assert.Contains(t, bodyString, expect)
	})

	t.Run("iframe manifest", func(t *testing.T) {
		reqURL := client.URL + "/plugins/com.mattermost.msteams-sync/iframe-manifest"
		resp, err := client.DoAPIRequest(context.Background(), http.MethodGet, reqURL, "", "")
		require.NoError(t, err, "cannot fetch url %s", reqURL)
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		require.Nil(t, err)
		require.Equal(t, 200, resp.StatusCode)

		// check if we have a valid zip
		zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
		require.NoError(t, err)

		expectedFilenames := []string{"manifest.json", "mm-logo-color.png", "mm-logo-outline.png"}
		count := 0

		// check the zip file contains the 3 files we expect
		for _, zipFile := range zipReader.File {
			count++
			assert.Contains(t, expectedFilenames, zipFile.Name)

			buf, err := readZipFile(zipFile)
			require.NoError(t, err, "cannot read zip file %s", zipFile.Name)

			switch zipFile.Name {
			case "manifest.json":
				assert.Contains(t, string(buf), "manifestVersion")
				assert.Contains(t, string(buf), url)
			case "mm-logo-color.png":
				assert.Equal(t, assets.LogoColorData, buf)
			case "mm-logo-outline.png":
				assert.Equal(t, assets.LogoOutlineData, buf)
			default:
				assert.Fail(t, "invalid file in zip: %s", zipFile.Name)
			}
		}
		assert.Equal(t, 3, count)
	})
}

func readZipFile(zipFile *zip.File) ([]byte, error) {
	rc, err := zipFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}
