package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCodeSnippet(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("failed to marshal codesnippet", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     "Invalid JSON",
		}
		message := "message"

		expectedOutput := "message"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("url is unexpected", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet"}`,
		}
		message := "message"

		expectedOutput := "message"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("failure retrieving code snippet", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
		}
		message := "message"

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("", errors.New("Error while retrieving code snippet"))

		expectedOutput := "message"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("code snippet for channel", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
		}
		message := "message"

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)

		expectedOutput := "message\n```go\nsnippet content\n```\n"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("code snippet for chat", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
		}
		message := "message"

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)

		expectedOutput := "message\n```go\nsnippet content\n```\n"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})
}

func TestHandleMessageReference(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("unable to marshal content", func(t *testing.T) {
		chatOrChannelID := model.NewId()

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     "Invalid JSON",
		}

		actualParentID := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID)
		assert.Empty(t, actualParentID)
	})

	t.Run("unknown message", func(t *testing.T) {
		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     `{"messageId": "` + messageID + `"}`,
		}

		actualParentID := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID)
		assert.Empty(t, actualParentID)
	})

	t.Run("successful lookup, no parent", func(t *testing.T) {
		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team)

		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		post := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		err = th.p.GetStore().LinkPosts(storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatOrChannelID,
			MSTeamsLastUpdateAt: time.Now(),
		})
		require.NoError(t, err)

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     `{"messageId": "` + messageID + `"}`,
		}

		expectedParentID := post.Id

		actualParentID := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID)
		assert.Equal(t, expectedParentID, actualParentID)
	})

	t.Run("successful lookup, with parent", func(t *testing.T) {
		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team)

		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		rootPost := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
		}
		err := th.p.apiClient.Post.CreatePost(rootPost)
		require.NoError(t, err)

		post := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
			RootId:    rootPost.Id,
		}
		err = th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		err = th.p.GetStore().LinkPosts(storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatOrChannelID,
			MSTeamsLastUpdateAt: time.Now(),
		})
		require.NoError(t, err)

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     `{"messageId": "` + messageID + `"}`,
		}

		expectedParentID := rootPost.Id

		actualParentID := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID)
		assert.Equal(t, expectedParentID, actualParentID)
	})
}

func TestHandleAttachments(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	assertFile := func(th *testHelper, t *testing.T, expectedName string, expectedBytes []byte, fileID string) {
		t.Helper()

		fileInfo, err := th.p.apiClient.File.GetInfo(fileID)
		require.NoError(t, err)
		assert.Equal(t, expectedName, fileInfo.Name)

		fileReader, err := th.p.apiClient.File.Get(fileID)
		require.NoError(t, err)
		fileBytes, err := io.ReadAll(fileReader)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, fileBytes)
	}

	t.Run("single file attachment, no existing file id", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					Name: "mock-name",
				},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{}

		th.appClientMock.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Once()
		th.appClientMock.On("GetFileContent", "mockDownloadURL").Return([]byte("abcde"), nil).Once()

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)
		assert.Equal(t, "message", newText)
		if assert.Len(t, attachmentIDs, 1) {
			assertFile(th, t, "mock-name", []byte("abcde"), attachmentIDs[0])
		}
		assert.Equal(t, "", parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("single file attachment, existing file id", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		existingFileInfo, err := th.p.apiClient.File.Upload(bytes.NewReader([]byte("12345")), "mock-name", channel.Id)
		require.NoError(t, err)

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					Name: "mock-name",
				},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{
			existingFileInfo.Id,
		}

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)
		assert.Equal(t, "message", newText)
		if assert.Len(t, attachmentIDs, 1) {
			assertFile(th, t, "mock-name", []byte("12345"), attachmentIDs[0])
		}
		assert.Equal(t, "", parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("multiple file attachments, no existing file id", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					Name: "mock-name-1",
				},
				{
					Name: "mock-name-2",
				},
				{
					Name: "mock-name-3",
				},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{}

		th.appClientMock.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Times(3)
		th.appClientMock.On("GetFileContent", "mockDownloadURL").Return([]byte("abcde"), nil).Times(3)

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)
		assert.Equal(t, "message", newText)
		if assert.Len(t, attachmentIDs, 3) {
			assertFile(th, t, "mock-name-1", []byte("abcde"), attachmentIDs[0])
			assertFile(th, t, "mock-name-2", []byte("abcde"), attachmentIDs[1])
			assertFile(th, t, "mock-name-3", []byte("abcde"), attachmentIDs[2])
		}
		assert.Equal(t, "", parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("multiple file attachments, some existing file ids", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		existingFileInfo1, err := th.p.apiClient.File.Upload(bytes.NewReader([]byte("12345")), "mock-name-1", channel.Id)
		require.NoError(t, err)

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					Name: "mock-name-1",
				},
				{
					Name: "mock-name-2",
				},
				{
					Name: "mock-name-3",
				},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{
			existingFileInfo1.Id,
		}

		th.appClientMock.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Times(2)
		th.appClientMock.On("GetFileContent", "mockDownloadURL").Return([]byte("abcde"), nil).Times(2)

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)
		assert.Equal(t, "message", newText)
		if assert.Len(t, attachmentIDs, 3) {
			assertFile(th, t, "mock-name-1", []byte("12345"), attachmentIDs[0])
			assertFile(th, t, "mock-name-2", []byte("abcde"), attachmentIDs[1])
			assertFile(th, t, "mock-name-3", []byte("abcde"), attachmentIDs[2])
		}
		assert.Equal(t, "", parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("multiple file attachments, all existing file ids", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		existingFileInfo1, err := th.p.apiClient.File.Upload(bytes.NewReader([]byte("12345")), "mock-name-1", channel.Id)
		require.NoError(t, err)

		existingFileInfo2, err := th.p.apiClient.File.Upload(bytes.NewReader([]byte("12345")), "mock-name-2", channel.Id)
		require.NoError(t, err)

		existingFileInfo3, err := th.p.apiClient.File.Upload(bytes.NewReader([]byte("12345")), "mock-name-3", channel.Id)
		require.NoError(t, err)

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					Name: "mock-name-1",
				},
				{
					Name: "mock-name-2",
				},
				{
					Name: "mock-name-3",
				},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{
			existingFileInfo1.Id,
			existingFileInfo2.Id,
			existingFileInfo3.Id,
		}

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)
		assert.Equal(t, "message", newText)
		if assert.Len(t, attachmentIDs, 3) {
			assertFile(th, t, "mock-name-1", []byte("12345"), attachmentIDs[0])
			assertFile(th, t, "mock-name-2", []byte("12345"), attachmentIDs[1])
			assertFile(th, t, "mock-name-3", []byte("12345"), attachmentIDs[2])
		}
		assert.Equal(t, "", parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("more than 10 attachments", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{Name: "mock-name-1"},
				{Name: "mock-name-2"},
				{Name: "mock-name-3"},
				{Name: "mock-name-4"},
				{Name: "mock-name-5"},
				{Name: "mock-name-6"},
				{Name: "mock-name-7"},
				{Name: "mock-name-8"},
				{Name: "mock-name-9"},
				{Name: "mock-name-10"},
				{Name: "mock-name-11"},
				{Name: "mock-name-12"},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{}

		th.appClientMock.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Times(10)
		th.appClientMock.On("GetFileContent", "mockDownloadURL").Return([]byte("abcde"), nil).Times(10)

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)
		assert.Equal(t, "message", newText)
		if assert.Len(t, attachmentIDs, 10) {
			for i := 0; i < 10; i++ {
				assertFile(th, t, fmt.Sprintf("mock-name-%d", i+1), []byte("abcde"), attachmentIDs[i])
			}
		}
		assert.Equal(t, "", parentID)
		assert.Equal(t, 2, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("code snippet", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team, WithMembers(user))

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					Name:        "mock-name",
					ContentType: "application/vnd.microsoft.card.codesnippet",
					Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
				},
			},
			ChatID:    model.NewId(),
			ChannelID: model.NewId(),
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{}

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)

		assert.Equal(t, `message
`+"```"+`go
snippet content
`+"```"+`
`, newText)
		assert.Len(t, attachmentIDs, 0)
		assert.Equal(t, "", parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})

	t.Run("message reference", func(t *testing.T) {
		th.Reset(t)

		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team)

		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		rootPost := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
		}
		err := th.p.apiClient.Post.CreatePost(rootPost)
		require.NoError(t, err)

		post := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
			RootId:    rootPost.Id,
		}
		err = th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		err = th.p.GetStore().LinkPosts(storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatOrChannelID,
			MSTeamsLastUpdateAt: time.Now(),
		})
		require.NoError(t, err)

		text := "message"
		message := &clientmodels.Message{
			Attachments: []clientmodels.Attachment{
				{
					ContentType: "messageReference",
					Content:     `{"messageId": "` + messageID + `"}`,
				},
			},
			ChatID:    "",
			ChannelID: chatOrChannelID,
		}
		chat := (*clientmodels.Chat)(nil)
		existingFileIDs := []string{}

		newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := th.p.activityHandler.handleAttachments(
			channel.Id,
			user.Id,
			text,
			message,
			chat,
			existingFileIDs,
		)

		assert.Equal(t, "message", newText)
		assert.Len(t, attachmentIDs, 0)
		assert.Equal(t, rootPost.Id, parentID)
		assert.Equal(t, 0, skippedFileAttachments)
		assert.False(t, errorsFound)
	})
}
