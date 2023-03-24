package testutils

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v6/model"
)

func GetInternalServerAppError(errorMsg string) *model.AppError {
	return &model.AppError{
		StatusCode:    http.StatusInternalServerError,
		DetailedError: errorMsg,
	}
}

func GetID() string {
	return "sfmq19kpztg5iy47ebe51hb31w"
}

func GetChannelID() string {
	return "bnqnzipmnir4zkkj95ggba5pde"
}

func GetPost() *model.Post {
	return &model.Post{
		Id:        GetID(),
		FileIds:   model.StringArray{GetID()},
		ChannelId: GetChannelID(),
		UserId:    GetID(),
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
