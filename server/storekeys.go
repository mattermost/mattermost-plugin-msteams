package main

func avatarKey(userID string) string {
	return "avatar_" + userID
}

func mattermostTeamsPostKey(postID string) string {
	return "mattermost_teams_" + postID
}

func teamsMattermostPostKey(postID string) string {
	return "teams_mattermost_" + postID
}

func teamsMattermostChatKey(chatID string) string {
	return "teams_mattermost_chat_" + chatID
}
