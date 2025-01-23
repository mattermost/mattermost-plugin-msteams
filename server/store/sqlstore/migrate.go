// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"embed"
	"fmt"

	"github.com/mattermost/morph"
	"github.com/mattermost/morph/drivers"
	"github.com/mattermost/morph/drivers/postgres"
	"github.com/mattermost/morph/sources/embedded"
)

const (
	migrationRemoteIDRequiredVersion         = 12
	migrationWhitelistedUsersRequiredVersion = 13
)

//go:embed migrations/*.sql
var Assets embed.FS

func (s *SQLStore) Migrate(remoteID string) error {
	driver, err := postgres.WithInstance(s.db)
	if err != nil {
		return fmt.Errorf("cannot create postgres driver: %w", err)
	}

	assetsList, err := Assets.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("cannot read assets dir: %w", err)
	}

	assetsNames := make([]string, len(assetsList))
	for i, entry := range assetsList {
		assetsNames[i] = entry.Name()
	}

	migrationAssets := &embedded.AssetSource{
		Names: assetsNames,
		AssetFunc: func(name string) ([]byte, error) {
			return Assets.ReadFile("migrations/" + name)
		},
	}

	src, err := embedded.WithInstance(migrationAssets)
	if err != nil {
		return err
	}

	opts := []morph.EngineOption{
		morph.WithLock("msteams-lock-key"),
		morph.SetMigrationTableName("msteamssync_schema_migrations"),
		morph.SetStatementTimeoutInSeconds(1000000),
	}

	s.api.LogDebug("Creating migration engine")
	engine, err := morph.New(context.Background(), driver, src, opts...)
	if err != nil {
		return err
	}
	defer func() {
		s.api.LogDebug("Closing migration engine")
		engine.Close()
	}()

	return s.runMigrationSequence(engine, driver, remoteID)
}

// runMigrationSequence executes all the migrations in order, both
// plain SQL and data migrations.
func (s *SQLStore) runMigrationSequence(engine *morph.Morph, driver drivers.Driver, remoteID string) error {
	if mErr := s.ensureMigrationsAppliedUpToVersion(engine, driver, migrationRemoteIDRequiredVersion); mErr != nil {
		return mErr
	}

	if mErr := s.ensureMigrationsAppliedUpToVersion(engine, driver, migrationWhitelistedUsersRequiredVersion); mErr != nil {
		return mErr
	}

	if err := s.runWhitelistedUsersMigration(); err != nil {
		return err
	}

	appliedMigrations, err := driver.AppliedMigrations()
	if err != nil {
		return err
	}

	s.api.LogDebug("== Applying all remaining migrations ====================",
		"current_version", len(appliedMigrations),
	)

	return engine.ApplyAll()
}

func (s *SQLStore) ensureMigrationsAppliedUpToVersion(engine *morph.Morph, driver drivers.Driver, version int) error {
	applied, err := driver.AppliedMigrations()
	if err != nil {
		return err
	}
	currentVersion := len(applied)

	s.api.LogDebug("== Ensuring migrations applied up to version ====================", "version", version, "current_version", currentVersion)

	// if the target version is below or equal to the current one, do
	// not migrate either because is not needed (both are equal) or
	// because it would downgrade the database (is below)
	if version <= currentVersion {
		s.api.LogDebug("-- There is no need of applying any migration --------------------")
		return nil
	}

	for _, migration := range applied {
		s.api.LogDebug("-- Found applied migration --------------------", "version", migration.Version, "name", migration.Name)
	}

	if _, err = engine.Apply(version - currentVersion); err != nil {
		return err
	}

	return nil
}
