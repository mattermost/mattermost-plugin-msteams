package sqlstore

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestRunMSTeamUserIDDedup(t *testing.T) {
	store, api := setupTestStore(t)
	assert := require.New(t)

	api.On("LogInfo", "Deleting duplicates").Once()

	/*
		Test matrix:

		|---------|------|------------------------------------------------------|
		| TeamsID | MMID | Comment                                              |
		|---------|------|------------------------------------------------------|
		| A       | 1    |                                                      |
		| A       | 2    | < will be kept because it's not a remote user        |
		| A       | 3    |                                                      |
		|---------|------|------------------------------------------------------|
		| B       | 1    | < will be kept because it was there first (createat) |
		| B       | 3    |                                                      |
		|---------|------|------------------------------------------------------|
		| C       | 1    | < not part of the dedup                              |
		|---------|------|------------------------------------------------------|
		| A       | 1    |                                                      |
		| B       | 2    |                                                      |
		| C       | 3    |                                                      |
		| D       | 4    | < not a remote user AND created first                |
		|---------|------|------------------------------------------------------|
	*/

	userID1Remote := model.NewId()
	userID2 := model.NewId()
	userID3Remote := model.NewId()
	userID4 := model.NewId()
	teamsUserA := "A"
	teamsUserB := "B"
	teamsUserC := "C"
	teamsUserD := "D"

	_, err := store.db.Exec("DROP INDEX IF EXISTS idx_msteamssync_users_msteamsuserid_unq")
	assert.NoError(err)
	defer store.createMSTeamsUserIdUniqueIndex()

	res, err := store.getQueryBuilder().Insert("users").
		Columns("id", "createat", "remoteid").
		Values(userID1Remote, 100, "remote-id").
		Values(userID2, 200, "").
		Values(userID3Remote, 300, "remote-id").
		Values(userID4, 10, "").
		Exec()
	assert.NoError(err)
	affected, err := res.RowsAffected()
	assert.NoError(err)
	assert.Equal(int64(4), affected)

	// ms teams user 1 will have all 3 users;
	// user1 and 3 should be removed after the dedup because user 2 is a real user
	res, err = store.getQueryBuilder().Insert(usersTableName).Columns("mmuserid", "msteamsuserid").
		Values(userID1Remote, teamsUserA).
		Values(userID2, teamsUserA).
		Values(userID3Remote, teamsUserA).
		Values(userID1Remote, teamsUserB).
		Values(userID3Remote, teamsUserB).
		Values(userID1Remote, teamsUserC).
		Values(userID1Remote, teamsUserD).
		Values(userID2, teamsUserD).
		Values(userID3Remote, teamsUserD).
		Values(userID4, teamsUserD).
		Exec()
	assert.NoError(err)
	affected, err = res.RowsAffected()
	assert.NoError(err)
	assert.Equal(int64(10), affected)

	err = store.runMSTeamUserIDDedup()
	assert.NoError(err)

	rows, err := store.getQueryBuilder().Select("mmuserid", "msteamsuserid").From(usersTableName).Query()
	assert.NoError(err)
	var found int
	for rows.Next() {
		var mmUserID, teamsUserID string
		err = rows.Scan(&mmUserID, &teamsUserID)
		assert.NoError(err)

		if teamsUserID == teamsUserA {
			assert.Equal(userID2, mmUserID)
			found++
		}
		if teamsUserID == teamsUserB {
			assert.Equal(userID1Remote, mmUserID)
			found++
		}
		if teamsUserID == teamsUserC {
			assert.Equal(userID1Remote, mmUserID)
			found++
		}
		if teamsUserID == teamsUserD {
			assert.Equal(userID4, mmUserID)
			found++
		}
	}
	assert.Equalf(4, found, "expected 4 rows to be found, but found %d", found)
}
