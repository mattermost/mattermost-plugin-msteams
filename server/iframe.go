package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mattermost/mattermost-plugin-msteams-sync/assets"
)

const (
	AppVersion          = "0.3.2"
	AppID               = "2028483e-7b45-4fe4-9237-cab8b5d4db5e"
	PackageName         = "com.mattermost.msteamsapp"
	TabAppID            = "c0d2e25c-0f6d-421c-8dcb-1702a00286d8"
	TabAppURI           = "api://%s/plugins/" + pluginID + "/iframe/c0d2e25c-0f6d-421c-8dcb-1702a00286d8"
	ManifestName        = "manifest.json"
	LogoColorFilename   = "mm-logo-color.png"
	LogoOutlineFilename = "mm-logo-outline.png"
)

// iFrame returns the iFrame HTML needed to host Mattermost within a MS Teams app.
func (a *API) iFrame(w http.ResponseWriter, _ *http.Request) {
	config := a.p.API.GetConfig()
	siteURL := *config.ServiceSettings.SiteURL
	if siteURL == "" {
		a.p.API.LogError("SiteURL cannot be empty for MS Teams iFrame")
		http.Error(w, "SiteURL is empty", http.StatusInternalServerError)
		return
	}

	html := strings.ReplaceAll(iFrameHTML, "{{SITE_URL}}", siteURL)

	w.Header().Set("Content-Type", "text/html")

	// set session cookie to indicate Mattermost is hosted in an iFrame, which allows
	// webapp to bypass "Where do you want to view this" page and set SameSite=none.
	http.SetCookie(w, &http.Cookie{
		Name:   "MMEMBED",
		Value:  "1",
		Path:   "/",
		Secure: true,
	})

	if _, err := w.Write([]byte(html)); err != nil {
		a.p.API.LogError("Unable to serve the iFrame", "Error", err.Error())
	}
}

// iFrameManifest returns the Mattermost for MS Teams app manifest as a zip file.
// This zip file can be imported as a MS Teams app.
func (a *API) iFrameManifest(w http.ResponseWriter, _ *http.Request) {
	config := a.p.API.GetConfig()
	siteURL := *config.ServiceSettings.SiteURL
	if siteURL == "" {
		a.p.API.LogError("SiteURL cannot be empty for MS Teams app manifest")
		http.Error(w, "SiteURL is empty", http.StatusInternalServerError)
		return
	}

	publicHostName, protocol, err := parseDomain(siteURL)
	if err != nil {
		a.p.API.LogError("SiteURL is invalid for MS Teams app manifest", "Error", err.Error())
		http.Error(w, "SiteURL is invalid: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tabURI := fmt.Sprintf(TabAppURI, publicHostName)

	manifest := strings.ReplaceAll(manifestJSON, "{{VERSION}}", AppVersion)
	manifest = strings.ReplaceAll(manifest, "{{APP_ID}}", AppID)
	manifest = strings.ReplaceAll(manifest, "{{PACKAGE_NAME}}", PackageName)
	manifest = strings.ReplaceAll(manifest, "{{PROTOCOL}}", protocol)
	manifest = strings.ReplaceAll(manifest, "{{PUBLIC_HOSTNAME}}", publicHostName)
	manifest = strings.ReplaceAll(manifest, "{{TAB_APP_ID}}", TabAppID)
	manifest = strings.ReplaceAll(manifest, "{{TAB_APP_URI}}", tabURI)
	manifest = strings.ReplaceAll(manifest, "{{PLUGIN_ID}}", pluginID)

	bufReader, err := createManifestZip(
		zipFile{name: ManifestName, data: []byte(manifest)},
		zipFile{name: LogoColorFilename, data: assets.LogoColorData},
		zipFile{name: LogoOutlineFilename, data: assets.LogoOutlineData},
	)
	if err != nil {
		a.p.API.LogError("Error generating app manifest", "Error", err.Error())
		http.Error(w, "Error generating app manifest", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=com.mattermost.msteamsapp.zip")

	if _, err := io.Copy(w, bufReader); err != nil {
		a.p.API.LogError("Unable to serve the app manifest", "Error", err.Error())
	}
}

func parseDomain(uri string) (string, string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	return u.Host, u.Scheme, nil
}

type zipFile struct {
	name string
	data []byte
}

func createManifestZip(files ...zipFile) (io.Reader, error) {
	buf := &bytes.Buffer{}

	w := zip.NewWriter(buf)
	defer w.Close()

	for _, zf := range files {
		fw, err := w.Create(zf.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(fw, bytes.NewReader(zf.data)); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

var iFrameHTML = `<!DOCTYPE html>
<html>

<head>
  <meta charset="UTF-8">
  <title>Mattermost</title>
  <meta name="viewport" content="width=device-width, height=device-height, initial-scale=1.0">
</head>

<body>
    <iframe 
		style="position:absolute;top:0px;width:100%;height:100vh;"
		src="{{SITE_URL}}" title="Mattermost">
	</iframe>
</body>

</html>
`

var manifestJSON = `{
	"$schema": "https://developer.microsoft.com/en-us/json-schemas/teams/v1.15/MicrosoftTeams.schema.json",
	"manifestVersion": "1.15",
	"id": "{{APP_ID}}",
	"version": "{{VERSION}}",
	"packageName": "{{PACKAGE_NAME}}",
	"developer": {
	  "name": "Mattermost",
	  "websiteUrl": "https://github.com/wiggin77/msteamsapp",
	  "privacyUrl": "https://mattermost.com/privacy-policy/",
	  "termsOfUseUrl": "https://mattermost.com/terms-of-use/"
	},
	"name": {
	  "short": "Mattermost for MS Teams",
	  "full": "Mattermost app for Microsoft Teams"
	},
	"description": {
	  "short": "Mattermost for MS Teams",
	  "full": "Mattermost app for Microsoft Teams"
	},
	"icons": {
	  "outline": "mm-logo-outline.png",
	  "color": "mm-logo-color.png"
	},
	"accentColor": "#FFFFFF",
	"configurableTabs": [],
	"staticTabs": [
	  {
		"entityId": "f607c5e9-7175-44ee-ba14-10e33a7b4c91",
		"name": "Mattermost",
		"contentUrl": "{{PROTOCOL}}://{{PUBLIC_HOSTNAME}}/plugins/{{PLUGIN_ID}}/iframe/mattermostTab?name={loginHint}&tenant={tid}&theme={theme}",
		"scopes": [
		  "personal"
		]
	  }
	],
	"bots": [],
	"connectors": [],
	"composeExtensions": [],
	"permissions": [
	  "identity",
	  "messageTeamMembers"
	],
	"validDomains": [
	  "{{PUBLIC_HOSTNAME}}"
	],
	"showLoadingIndicator": false,
	"isFullScreen": true,
	"webApplicationInfo": {
	  "id": "{{TAB_APP_ID}}",
	  "resource": "{{TAB_APP_URI}}"
	}
  }
`
