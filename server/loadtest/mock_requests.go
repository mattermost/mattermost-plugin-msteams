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

func simulatePostToChat(data PostToChatJob) {
	var activities *MSActivities
	var err error
	if strings.HasPrefix(data.channelId, "ms-dm-") {
		activities, err = buildPostActivityForDM(data)
	} else if strings.HasPrefix(data.channelId, "ms-gm-") {
		activities, err = buildPostActivityForGM(data)
	} else {
		err = fmt.Errorf("simulate post channel is not supported. type = %s", data.channelId)
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
	maxIncoming := Settings.maxIncomingPosts - Settings.minIncomingPosts
	numberOfRequests := rand.Intn(maxIncoming+1) + Settings.minIncomingPosts

	for i := 1; i <= numberOfRequests; i++ {
		job := PostToChatJob{
			channelId: channelId,
			msUserId:  msUserId,
			message:   message,
			count:     i,
			total:     numberOfRequests,
		}
		SimulateQueue <- job
	}
}
