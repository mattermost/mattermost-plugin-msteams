package loadtest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enescakir/emoji"
	mark "github.com/mattermost/mattermost-plugin-msteams/server/markdown"
	"github.com/mattermost/mattermost/server/public/model"
	"gitlab.com/golang-commonmark/markdown"
)

type boolegen struct {
	src       rand.Source
	cache     int64
	remaining int
}

func (b *boolegen) Bool() bool {
	if b.remaining == 0 {
		b.cache, b.remaining = b.src.Int63(), 63
	}

	result := b.cache&0x01 == 1
	b.cache >>= 1
	b.remaining--

	return result
}

var AttachmentsSync sync.Map
var Boolgen *boolegen

func uncompressRequestBody(req *http.Request) ([]byte, error) {
	gzippedBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	buf := bytes.NewReader(gzippedBody)
	// Create a new reader for the gzip data
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	uncompressedBody, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return uncompressedBody, nil
}

func requestBodyToMap(req *http.Request) (map[string]interface{}, error) {
	uncompressedBody, err := uncompressRequestBody(req)
	if err != nil {
		return nil, err
	}
	var bodyJson map[string]interface{}
	err = json.Unmarshal(uncompressedBody, &bodyJson)
	if err != nil {
		return nil, err
	}

	return bodyJson, nil
}

func getPostContentAsMD(bodyMap map[string]interface{}) string {
	body := bodyMap["body"].(map[string]interface{})
	return mark.ConvertToMD(body["content"].(string))
}

func getAttachmentFromContent(body map[string]interface{}) []interface{} {
	attachment, ok := body["attachments"]
	if !ok {
		return nil
	}
	return attachment.([]interface{})
}

func getUserDataFromAuthHeader(req *http.Request) (string, string) {
	findIndex := func(array []string, value string) int {
		for i, n := range array {
			if strings.Contains(n, value) {
				return i
			}
		}
		return -1
	}

	authHeader := req.Header["Authorization"]
	bearerIndex := findIndex(authHeader, "Bearer")
	token := ""
	if bearerIndex >= 0 {
		token = authHeader[bearerIndex][7:]
	}

	mmUserId := ""
	msUserId := ""
	if data, ok := Settings.GetUserTokenData(token); ok {
		mmUserId = data.UserId
		msUserId = data.TeamsUserId
	}
	return mmUserId, msUserId
}

func getOtherUserFromChannelId(channelId, userId string) string {
	userIds := strings.Split(strings.Replace(channelId, "ms-dm-", "", 1), "__")
	for _, n := range userIds {
		if n == userId {
			continue
		}

		return n
	}
	return ""
}

func getRandomUserFromChannelId(channelId, userId string) string {
	userIds := strings.Split(strings.Replace(channelId, "ms-gm-", "", 1), "__")
	idsWithoutUserId := slices.DeleteFunc(userIds, func(id string) bool {
		return userId == id
	})
	randInt := rand.Intn(len(idsWithoutUserId))
	return userIds[randInt]
}

func getGMNameFromIds(userIds []string) string {
	sort.Strings(userIds)
	return strings.Join(userIds, "__")
}

func getHtmlFromMD(message string) string {
	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false), markdown.LangPrefix("CodeMirror language-"))
	return md.RenderToString([]byte(emoji.Parse(message)))
}

func buildAttachmentForMessage(rooId, otherUserId string) []interface{} {
	content := map[string]any{
		"messageId":      rooId,
		"messagePreview": "<p>This is a reply</p>",
		"messageSender": map[string]any{
			"user": map[string]any{
				"userIdentityType": "aadUser",
				"id":               otherUserId,
				"displayName":      otherUserId,
			},
		},
	}

	contentString, err := json.Marshal(content)
	if err != nil {
		return nil
	}

	attachments := []any{
		map[string]any{
			"id":          rooId,
			"contentType": "messageReference",
			"content":     string(contentString),
		},
	}

	return attachments
}

func buildMessageContent(channelId, msgId, message, otherUserId string) MSContent {
	text := getHtmlFromMD(message)
	stored, loaded := AttachmentsSync.LoadAndDelete(msgId)
	attachments := []any{}
	if loaded {
		attachments = stored.([]interface{})
	}

	content := MSContent{
		ID:                   msgId,
		Etag:                 msgId,
		MessageType:          "message",
		CreatedDateTime:      time.Now(),
		LastModifiedDateTime: time.Now(),
		ChatID:               channelId,
		Importance:           "normal",
		Locale:               "en-us",
		From: MSContentFrom{
			User: MSContentFromUser{
				ID:               otherUserId,
				DisplayName:      otherUserId,
				UserIdentityType: "aadUser",
			},
		},
		Body: MSContentBody{
			ContentType: "text",
			Content:     text,
		},
		Attachments:    attachments,
		Mentions:       []any{},
		Reactions:      []any{},
		Replies:        []any{},
		HostedContents: []any{},
	}

	if len(attachments) > 0 {
		log("CREATE RANDOM ATTACHMENT", "msgId", msgId, "content", content)
	}

	return content
}

func buildPostActivityForDM(data PostToChatJob) (*MSActivities, error) {
	var contentJson []byte
	var err error
	msgId := model.NewId()
	text := fmt.Sprintf("%s\nAnswer #%d", data.message, data.count)
	AttachmentsSync.Store(msgId, data.attachments)

	if Settings.simulateIncomingPosts {
		otherUserId := "ms_teams-" + getOtherUserFromChannelId(data.channelId, data.msUserId)
		content := buildMessageContent(data.channelId, msgId, text, otherUserId)

		contentJson, err = json.Marshal(content)
		if err != nil {
			return nil, err
		}
	} else {
		msgId = fmt.Sprintf("%s{{{%s}}}", msgId, text)
	}

	activities := &MSActivities{
		Value: []MSActivity{
			{
				SubscriptionID:                 "msteams_subscriptions_id",     // hard coded as included in the response for init subscriptions
				SubscriptionExpirationDateTime: "2036-11-20T18:23:45.9356913Z", // hard coded as included in the response for init subscriptions
				ClientState:                    Settings.webhookSecret,
				ChangeType:                     "created",
				Resource:                       "chats('" + data.channelId + "')/messages('" + msgId + "')",
				TenantID:                       Settings.tenantId,
				Content:                        contentJson,
			},
		},
	}

	return activities, nil
}

func buildPostActivityForGM(data PostToChatJob) (*MSActivities, error) {
	var contentJson []byte
	var err error
	msgId := model.NewId()
	text := fmt.Sprintf("%s\nAnswer #%d", data.message, data.count)
	if len(data.attachments) > 0 {
		AttachmentsSync.Store(msgId, data.attachments)
	} else if Boolgen.Bool() {
		attachments := buildAttachmentForMessage(data.rootId, data.msUserId)
		AttachmentsSync.Store(msgId, attachments)
	}
	if Settings.simulateIncomingPosts {
		otherUserId := "ms_teams-" + getRandomUserFromChannelId(data.channelId, strings.Replace(data.msUserId, "ms_teams-", "", 1))
		content := buildMessageContent(data.channelId, msgId, text, otherUserId)

		contentJson, err = json.Marshal(content)
		if err != nil {
			return nil, err
		}
	} else {
		msgId = fmt.Sprintf("%s{{{%s}}}", msgId, text)
	}

	activities := &MSActivities{
		Value: []MSActivity{
			{
				SubscriptionID:                 "msteams_subscriptions_id",     // hard coded as included in the response for init subscriptions
				SubscriptionExpirationDateTime: "2036-11-20T18:23:45.9356913Z", // hard coded as included in the response for init subscriptions
				ClientState:                    Settings.webhookSecret,
				ChangeType:                     "created",
				Resource:                       "chats('" + data.channelId + "')/messages('" + msgId + "')",
				TenantID:                       Settings.tenantId,
				Content:                        contentJson,
			},
		},
	}

	return activities, nil
}
