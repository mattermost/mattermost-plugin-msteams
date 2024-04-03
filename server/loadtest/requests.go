package loadtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type MSContentFromUser struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	UserIdentityType string `json:"userIdentityType"`
}

type MSContentFrom struct {
	Application  any               `json:"application"`
	Device       any               `json:"device"`
	User         MSContentFromUser `json:"user"`
	Conversation any               `json:"conversation"`
}

type MSContentBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type MSContent struct {
	ID                   string        `json:"id"`
	ReplyToID            any           `json:"replyToId"`
	Etag                 string        `json:"etag"`
	MessageType          string        `json:"messageType"`
	CreatedDateTime      time.Time     `json:"createdDateTime"`
	LastModifiedDateTime time.Time     `json:"lastModifiedDateTime"`
	LastEditedDateTime   any           `json:"lastEditedDateTime"`
	DeletedDateTime      any           `json:"deletedDateTime"`
	Subject              any           `json:"subject"`
	Summary              any           `json:"summary"`
	ChatID               string        `json:"chatId"`
	Importance           string        `json:"importance"`
	Locale               string        `json:"locale"`
	WebURL               any           `json:"webUrl"`
	From                 MSContentFrom `json:"from"`
	Body                 MSContentBody `json:"body"`
	ChannelIdentity      any           `json:"channelIdentity"`
	Attachments          []any         `json:"attachments"`
	Mentions             []any         `json:"mentions"`
	PolicyViolation      any           `json:"policyViolation"`
	Reactions            []any         `json:"reactions"`
	Replies              []any         `json:"replies"`
	HostedContents       []any         `json:"hostedContents"`
}

type MSActivity struct {
	SubscriptionID                 string `json:"subscriptionId"`
	ChangeType                     string `json:"changeType"`
	ClientState                    string `json:"clientState"`
	SubscriptionExpirationDateTime string `json:"subscriptionExpirationDateTime"`
	Resource                       string `json:"resource"`
	Content                        []byte `json:"content"`
	TenantID                       string `json:"tenantId"`
}

type MSActivities struct {
	Value []MSActivity `json:"value"`
}

func simulatePostToChat(channelId, msUserId, message string, count int) {
	var activities *MSActivities
	var err error
	if strings.HasPrefix(channelId, "ms-dm-") {
		activities, err = buildPostActivityForDM(channelId, msUserId, message, count)
	} else if strings.HasPrefix(channelId, "ms-gm-") {
		activities, err = buildPostActivityForGM(channelId, msUserId, message, count)
	} else {
		err = fmt.Errorf("simulate post channel is not supported. type = %s", channelId)
	}

	if err != nil {
		log("simulatePostToChat failed", "error", err)
		return
	}

	randInt := rand.Int63n(5)
	time.Sleep(time.Duration(randInt) * time.Second)
	body, err := json.Marshal(activities)
	if err != nil {
		log("simulatePostToChat failed", "error", err)
		return
	}
	bodyReader := bytes.NewReader(body)
	requestUrl := fmt.Sprintf("%schanges", Settings.baseUrl)
	req, err := http.NewRequest(http.MethodPost, requestUrl, bodyReader)
	if err != nil {
		log("simulatePostToChat failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log("simulatePostToChat failed", "error", err)
	}
	defer resp.Body.Close()
}

func simulatePostsToChat(channelId, msUserId, message string) {
	randInt := rand.Intn(Settings.maxIncomingPosts)
	log("simulating incoming posts", "count", randInt, "of_max", Settings.maxIncomingPosts)
	for i := 1; i <= randInt; i++ {
		go simulatePostToChat(channelId, msUserId, message, i)
	}
}
