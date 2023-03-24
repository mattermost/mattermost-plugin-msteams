package testutils

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v6/model"
)

func GetChannelID() string {
	return "bnqnzipmnir4zkkj95ggba5pde"
}

func GetUserID() string {
	return "sfmq19kpztg5iy47ebe51hb31w"
}

func GetPostID() string {
	return "qwifdnaootmgkerodfdmwo"
}

func GetPost() *model.Post {
	return &model.Post{
		Id: GetPostID(),
	}
}

func GetInternalServerAppError(errorMsg string) *model.AppError {
	return &model.AppError{
		StatusCode:    http.StatusInternalServerError,
		DetailedError: errorMsg,
	}
}
