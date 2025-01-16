// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
)

func TestDescribeResourceAccessType(t *testing.T) {
	assert.Equal(t, "Scope (Delegated)", describeResourceAccessType(clientmodels.ResourceAccess{
		ID:   model.NewId(),
		Type: ResourceAccessTypeScope,
	}))
	assert.Equal(t, "Role (Application)", describeResourceAccessType(clientmodels.ResourceAccess{
		ID:   model.NewId(),
		Type: ResourceAccessTypeRole,
	}))
	assert.Equal(t, "Unknown", describeResourceAccessType(clientmodels.ResourceAccess{
		ID:   model.NewId(),
		Type: "Unknown",
	}))
}

func TestCheckPermissions(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("no permissions", func(t *testing.T) {
		th.Reset(t)

		var app clientmodels.App

		missing, redundant := th.p.checkPermissions(&app)

		assert.Equal(t, getExpectedPermissions(), missing)
		assert.Empty(t, redundant)
	})

	t.Run("missing and redundant permissions", func(t *testing.T) {
		th.Reset(t)

		var app clientmodels.App

		var changedResourceAccess clientmodels.ResourceAccess
		for i, expectedPermission := range getExpectedPermissions() {
			if i == 0 {
				// Skip the first permission altogether
				continue
			} else if i == 1 {
				// Change the type of the second permission
				changedResourceAccess = expectedPermission.ResourceAccess
				if changedResourceAccess.Type == ResourceAccessTypeScope {
					changedResourceAccess.Type = ResourceAccessTypeRole
				} else {
					changedResourceAccess.Type = ResourceAccessTypeScope
				}
				app.RequiredResources = append(app.RequiredResources, changedResourceAccess)
			} else {
				app.RequiredResources = append(app.RequiredResources, expectedPermission.ResourceAccess)
			}
		}

		// Add an extra permission beyond the changed one above.
		extraResourceAccess := clientmodels.ResourceAccess{
			ID:   model.NewId(),
			Type: ResourceAccessTypeRole,
		}
		app.RequiredResources = append(app.RequiredResources, extraResourceAccess)

		missing, redundant := th.p.checkPermissions(&app)
		assert.Equal(t, []expectedPermission{
			getExpectedPermissions()[0],
			getExpectedPermissions()[1],
		}, missing)
		assert.Equal(t, []clientmodels.ResourceAccess{
			changedResourceAccess,
			extraResourceAccess,
		}, redundant)
	})

	t.Run("expected permissions", func(t *testing.T) {
		th.Reset(t)

		var app clientmodels.App

		for _, expectedPermission := range getExpectedPermissions() {
			app.RequiredResources = append(app.RequiredResources, expectedPermission.ResourceAccess)
		}

		missing, redundant := th.p.checkPermissions(&app)
		assert.Empty(t, missing)
		assert.Empty(t, redundant)
	})
}
