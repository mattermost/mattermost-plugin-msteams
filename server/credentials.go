package main

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

const (
	ResourceAccessTypeScope = "Scope"
	ResourceAccessTypeRole  = "Role"
)

type expectedPermission struct {
	Name           string
	ResourceAccess clientmodels.ResourceAccess
}

// getResourceAccessKey makes a map key for the resource access that simplifies checking
// for the resource access in question. (Technically, we could use the struct itself, but
// this insulates us from unexpected upstream changes.)
func getResourceAccessKey(resourceAccess clientmodels.ResourceAccess) string {
	return fmt.Sprintf("%s+%s", resourceAccess.ID, resourceAccess.Type)
}

// describeResourceAccessType annotates the resource access type with the user facing term
// shown in the Azure Tenant UI (Application vs. Delegated).
func describeResourceAccessType(resourceAccess clientmodels.ResourceAccess) string {
	switch resourceAccess.Type {
	case ResourceAccessTypeRole:
		return "Role (Application)"
	case ResourceAccessTypeScope:
		return "Scope (Delegated)"
	default:
		return resourceAccess.Type
	}
}

// getExpectedPermissions returns the set of expected permissions, keyed by the
// name the enduser would expect to see in the Azure tenant.
func getExpectedPermissions() []expectedPermission {
	return []expectedPermission{
		{
			Name: "https://graph.microsoft.com/Chat.Read",
			ResourceAccess: clientmodels.ResourceAccess{
				ID:   "f501c180-9344-439a-bca0-6cbf209fd270",
				Type: "Scope",
			},
		},
		{
			Name: "https://graph.microsoft.com/ChatMessage.Read",
			ResourceAccess: clientmodels.ResourceAccess{
				ID:   "cdcdac3a-fd45-410d-83ef-554db620e5c7",
				Type: "Scope",
			},
		},
		{
			Name: "https://graph.microsoft.com/Files.Read.All",
			ResourceAccess: clientmodels.ResourceAccess{
				ID:   "df85f4d6-205c-4ac5-a5ea-6bf408dba283",
				Type: "Scope",
			},
		},
		{
			Name: "https://graph.microsoft.com/offline_access",
			ResourceAccess: clientmodels.ResourceAccess{
				ID:   "7427e0e9-2fba-42fe-b0c0-848c9e6a8182",
				Type: "Scope",
			},
		},
		{
			Name: "https://graph.microsoft.com/User.Read",
			ResourceAccess: clientmodels.ResourceAccess{
				ID:   "e1fe6dd8-ba31-4d61-89e7-88639da4683d",
				Type: "Scope",
			},
		},
		{
			Name: "https://graph.microsoft.com/Chat.Read.All",
			ResourceAccess: clientmodels.ResourceAccess{
				ID:   "6b7d71aa-70aa-4810-a8d9-5d9fb2830017",
				Type: "Role",
			},
		},
	}
}

func (p *Plugin) checkCredentials() {
	defer func() {
		if r := recover(); r != nil {
			p.GetMetrics().ObserveGoroutineFailure()
			p.API.LogError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	done := p.GetMetrics().ObserveWorker(metrics.WorkerCheckCredentials)
	defer done()

	p.API.LogInfo("Running the check credentials job")

	app, err := p.GetClientForApp().GetApp(p.getConfiguration().ClientID)
	if err != nil {
		p.API.LogWarn("Failed to get app credentials", "error", err.Error())
		return
	}

	credentials := app.Credentials

	// We sort by earliest end date to cover the unlikely event we encounter two credentials
	// with the same hint when reporting the single metric below.
	sort.SliceStable(credentials, func(i, j int) bool {
		return credentials[i].EndDateTime.Before(credentials[j].EndDateTime)
	})

	found := false
	for _, credential := range credentials {
		if strings.HasPrefix(p.getConfiguration().ClientSecret, credential.Hint) {
			p.API.LogInfo("Found matching credential", "credential_name", credential.Name, "credential_id", credential.ID, "credential_end_date_time", credential.EndDateTime)

			if !found {
				// Report the first one that matches the hint.
				p.GetMetrics().ObserveClientSecretEndDateTime(credential.EndDateTime)
			} else {
				// If we happen to get more than one with the same hint, we'll have reported the metric of the
				// earlier one by virtue of the sort above, and we'll have the extra metadata we need in the logs.
				p.API.LogWarn("Found more than one secret with same hint", "credential_id", credential.ID)
			}

			// Note that we keep going to log all the credentials found.
			found = true
		} else {
			p.API.LogInfo("Found other credential", "credential_name", credential.Name, "credential_id", credential.ID, "credential_end_date_time", credential.EndDateTime)
		}
	}

	if !found {
		p.API.LogWarn("Failed to find credential matching configuration")
		p.GetMetrics().ObserveClientSecretEndDateTime(time.Time{})
	}

	missingPermissions, redundantResourceAccess := p.checkPermissions(app)
	for _, permission := range missingPermissions {
		p.API.LogWarn(
			"Application missing required API Permission",
			"permission", permission.Name,
			"resource_id", permission.ResourceAccess.ID,
			"type", describeResourceAccessType(permission.ResourceAccess),
			"application_id", p.getConfiguration().ClientID,
		)
	}

	for _, resourceAccess := range redundantResourceAccess {
		p.API.LogWarn(
			"Application has redundant API Permission",
			"resource_id", resourceAccess.ID,
			"type", describeResourceAccessType(resourceAccess),
			"application_id", p.getConfiguration().ClientID,
		)
	}
}

func (p *Plugin) checkPermissions(app *clientmodels.App) ([]expectedPermission, []clientmodels.ResourceAccess) {
	// Build a map and log what we find at the same time.
	actualRequiredResources := make(map[string]clientmodels.ResourceAccess)
	for _, requiredResource := range app.RequiredResources {
		actualRequiredResources[getResourceAccessKey(requiredResource)] = requiredResource
		p.API.LogDebug(
			"Found API Permission",
			"resource_id", requiredResource.ID,
			"type", describeResourceAccessType(requiredResource),
			"application_id", p.getConfiguration().ClientID,
		)
	}

	expectedPermissions := getExpectedPermissions()
	expectedPermissionsMap := make(map[string]expectedPermission, len(expectedPermissions))
	for _, expectedPermission := range expectedPermissions {
		expectedPermissionsMap[getResourceAccessKey(expectedPermission.ResourceAccess)] = expectedPermission
	}

	var missing []expectedPermission
	var redundant []clientmodels.ResourceAccess

	// Verify all expected permissions are present.
	for _, permission := range expectedPermissions {
		if _, ok := actualRequiredResources[getResourceAccessKey(permission.ResourceAccess)]; !ok {
			missing = append(missing, permission)
		}
	}

	// Check for unnecessary permissions.
	for _, requiredResource := range app.RequiredResources {
		if _, ok := expectedPermissionsMap[getResourceAccessKey(requiredResource)]; !ok {
			redundant = append(redundant, requiredResource)
		}
	}

	return missing, redundant
}
