package store

func channelsLinkedKey(channelID string) string {
	return "channelsLinked_" + channelID
}

func channelsLinkedByMSTeamsKey(teamID, channelID string) string {
	return "channelsLinked_" + teamID + ":" + channelID
}

func avatarKey(userID string) string {
	return "avatar_" + userID
}

func mattermostTeamsPostKey(postID string) string {
	return "mattermost_teams_" + postID
}

func teamsMattermostPostKey(postID string) string {
	return "teams_mattermost_" + postID
}

func teamsMattermostUserKey(userID string) string {
	return "teams_mattermost_user_" + userID
}

func mattermostTeamsUserKey(userID string) string {
	return "mattermost_teams_user_" + userID
}

func teamsMattermostChatKey(chatID string) string {
	return "teams_mattermost_chat_" + chatID
}

// TODO: Add lodash at the end
func tokenForMattermostUserKey(userID string) string {
	return "token_for_mm_user" + userID
}

// TODO: Add lodash at the end
func tokenForTeamsUserKey(userID string) string {
	return "token_for_teams_user" + userID
}
