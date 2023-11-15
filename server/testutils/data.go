package testutils

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
)

func GetTeamID() string {
	return "pqoeurndhroajdemq4nfmw"
}

func GetChannelID() string {
	return "bnqnzipmnir4zkkj95ggba5pde"
}

func GetSenderID() string {
	return "pqoejrn65psweomewmosaqr"
}

func GetUserID() string {
	return "sfmq19kpztg5iy47ebe51hb31w"
}

func GetTeamsUserID() string {
	return "rioegneonqimomsoqwiew3qeo"
}

func GetMessageID() string {
	return "dsdfonreoapwer4onebfdr"
}

func GetReplyID() string {
	return "pqoeurndhroajdemq4nfmw"
}

func GetChatID() string {
	return "qplsnwere9nurernidteoqw"
}

func GetMattermostID() string {
	return "qwifdnaootmgkerodfdmwo"
}

func GetPostID() string {
	return "qwifdnaootmgkerodfdmwo"
}

func GetInternalServerAppError(errorMsg string) *model.AppError {
	return &model.AppError{
		StatusCode:    http.StatusInternalServerError,
		Message:       errorMsg,
		DetailedError: errorMsg,
		Id:            GetID(),
	}
}

func GetID() string {
	return "sfmq19kpztg5iy47ebe51hb31w"
}

func GetMSTeamsChannelID() string {
	return "qplsnwere9nurernidteoqw"
}

func GetPost(channelID, userID string, createAt int64) *model.Post {
	return &model.Post{
		Id:        GetID(),
		FileIds:   model.StringArray{GetID()},
		ChannelId: channelID,
		UserId:    userID,
		CreateAt:  createAt,
	}
}

func GetChannel(channelType model.ChannelType) *model.Channel {
	return &model.Channel{
		Id:   GetChannelID(),
		Type: channelType,
	}
}

func GetChannelMembers(count int) model.ChannelMembers {
	channelMembers := model.ChannelMembers{}
	for i := 0; i < count; i++ {
		channelMembers = append(channelMembers, model.ChannelMember{
			UserId:    GetID(),
			ChannelId: GetChannelID(),
		})
	}

	return channelMembers
}

func GetUser(role, email string) *model.User {
	return &model.User{
		Id:       GetID(),
		Username: "test-user",
		Roles:    role,
		Email:    email,
	}
}

func GetReaction() *model.Reaction {
	return &model.Reaction{
		EmojiName: "+1",
		UserId:    GetID(),
		PostId:    GetID(),
		ChannelId: GetChannelID(),
	}
}

func GetTeamsTeamID() string {
	return "test-teams-team-qplsnwere9nurernidte"
}

func GetTeamsChannelID() string {
	return "test-teams-channel"
}

func GetChannelLinks(count int) []*storemodels.ChannelLink {
	var links []*storemodels.ChannelLink
	for i := 0; i < count; i++ {
		links = append(links, &storemodels.ChannelLink{
			MattermostTeamID:      GetTeamID(),
			MattermostChannelID:   GetChannelID(),
			MattermostTeamName:    "Test MM team",
			MattermostChannelName: "Test MM channel",
			MSTeamsTeamID:         GetTeamsTeamID(),
			MSTeamsChannelID:      GetTeamsChannelID(),
		})
	}

	return links
}

func GetConnectedUsers(count int) []*storemodels.ConnectedUser {
	var connectedUsers []*storemodels.ConnectedUser
	for i := 0; i < count; i++ {
		connectedUsers = append(connectedUsers, &storemodels.ConnectedUser{
			MattermostUserID: GetUserID(),
			TeamsUserID:      GetTeamsUserID(),
		})
	}

	return connectedUsers
}

func GetFileInfo() *model.FileInfo {
	return &model.FileInfo{
		Id:       GetID(),
		Name:     "mockFile.Name.txt",
		Size:     1,
		MimeType: "mockMimeType",
	}
}

func GetPostFromTeamsMessage(createAt int64) *model.Post {
	return &model.Post{
		UserId:    GetUserID(),
		ChannelId: GetChannelID(),
		Message:   "mockText",
		CreateAt:  createAt,
		Props: model.StringInterface{
			"msteams_sync_mock-BotUserID": true,
		},
		FileIds: model.StringArray{},
	}
}

func GetChannelLink() *storemodels.ChannelLink {
	return &storemodels.ChannelLink{
		MattermostTeamID:    GetTeamID(),
		MattermostChannelID: GetChannelID(),
		MSTeamsTeamID:       GetTeamsTeamID(),
		MSTeamsChannelID:    GetTeamsChannelID(),
	}
}

func GetLinkChannelsPayload(teamID, channelID, msTeamsTeamID, msTeamsChannelID string) string {
	return fmt.Sprintf(`{
		"mattermostTeamID":"%s",
		"mattermostChannelID":"%s",
		"msTeamsTeamID":"%s",
		"msTeamsChannelID":"%s"
	}`, teamID, channelID, msTeamsTeamID, msTeamsChannelID)
}

func GetTestEmail() string {
	return "unknown-user@msteamssync"
}

func GetMockTime() time.Time {
	mockTime, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2023 at 4:00pm (MST)")
	return mockTime
}

func GetEphemeralPost(userID, channelID, message string) *model.Post {
	return &model.Post{
		UserId:    userID,
		ChannelId: channelID,
		Message:   message,
	}
}

func GetGlobalSubscription(subscriptionID string, expiresOn time.Time) storemodels.GlobalSubscription {
	return storemodels.GlobalSubscription{
		SubscriptionID: subscriptionID,
		Type:           "allChats",
		Secret:         "secret",
		ExpiresOn:      expiresOn,
	}
}

func GetChannelSubscription(subscriptionID, teamID, channelID string, expiresOn time.Time) storemodels.ChannelSubscription {
	return storemodels.ChannelSubscription{
		SubscriptionID: subscriptionID,
		TeamID:         teamID,
		ChannelID:      channelID,
		Secret:         "secret",
		ExpiresOn:      expiresOn,
	}
}

func GetChatSubscription(subscriptionID, userID string, expiresOn time.Time) storemodels.ChatSubscription {
	return storemodels.ChatSubscription{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Secret:         "secret",
		ExpiresOn:      expiresOn,
	}
}
