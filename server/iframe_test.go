package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-msteams-sync/assets"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
)

func TestIFrame(t *testing.T) {
	plugin := newTestPlugin(t)
	plugin.API = &plugintest.API{}

	config := &model.Config{}
	config.SetDefaults()
	config.ServiceSettings.SiteURL = model.NewString(model.ServiceSettingsDefaultSiteURL)
	plugin.API.(*plugintest.API).On("GetConfig").Return(config).Times(1)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/iframe/mattermostTab", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	require.NotNil(t, result)
	defer result.Body.Close()

	bodyBytes, err := io.ReadAll(result.Body)
	require.Nil(t, err)
	bodyString := string(bodyBytes)

	require.Equal(t, 200, result.StatusCode)

	assert.Contains(t, bodyString, "<!DOCTYPE html>")

	expect := fmt.Sprintf(`src="%s" title="Mattermost"`, model.ServiceSettingsDefaultSiteURL)
	assert.Contains(t, bodyString, expect)
}

func TestIFrameManifest(t *testing.T) {
	plugin := newTestPlugin(t)
	plugin.API = &plugintest.API{}

	config := &model.Config{}
	config.SetDefaults()
	config.ServiceSettings.SiteURL = model.NewString(model.ServiceSettingsDefaultSiteURL)
	plugin.API.(*plugintest.API).On("GetConfig").Return(config).Times(1)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/iframe-manifest", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	require.NotNil(t, result)
	defer result.Body.Close()

	bodyBytes, err := io.ReadAll(result.Body)
	require.Nil(t, err)
	require.Equal(t, 200, result.StatusCode)

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
		case ManifestName:
			assert.Contains(t, string(buf), "manifestVersion")
			assert.Contains(t, string(buf), model.ServiceSettingsDefaultSiteURL)
		case LogoColorFilename:
			assert.Equal(t, assets.LogoColorData, buf)
		case LogoOutlineFilename:
			assert.Equal(t, assets.LogoOutlineData, buf)
		default:
			assert.Fail(t, "invalid file in zip: %s", zipFile.Name)
		}
	}
	assert.Equal(t, 3, count)
}

func readZipFile(zipFile *zip.File) ([]byte, error) {
	rc, err := zipFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return ioutil.ReadAll(rc)
}
