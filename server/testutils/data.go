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
