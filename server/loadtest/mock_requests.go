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
	max := Settings.maxIncomingPosts - Settings.minIncomingPosts
	randInt := rand.Intn(max+1) + Settings.minIncomingPosts
	log("simulating incoming posts", "count", randInt, "min", Settings.minIncomingPosts, "max", Settings.maxIncomingPosts)
	for i := 1; i <= randInt; i++ {
		go simulatePostToChat(channelId, msUserId, message, i)
	}
}
