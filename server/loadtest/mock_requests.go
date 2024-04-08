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

var (
	maxRoutines           int
	simulatedRequestsChan chan int
	simulatedRequests     int
)

func init() {
	maxRoutines = 1000
	simulatedRequests = 0
	simulatedRequestsChan = make(chan int)

	startCount := func() {
		for count := range simulatedRequestsChan {
			simulatedRequests += count
			log("simulating requests", "count", simulatedRequests)
		}
	}

	go startCount()
}

func simulatePostToChat(channelId, msUserId, message string, count, total int) {
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

	if count == total {
		simulatedRequestsChan <- (total * -1)
	}
}

func simulatePostsToChat(channelId, msUserId, message string) {
	maxIncoming := Settings.maxIncomingPosts - Settings.minIncomingPosts
	routinesLeft := maxRoutines - simulatedRequests

	if routinesLeft > 0 {
		maxIncoming = minOf(maxIncoming, routinesLeft)
		numberOfRequests := rand.Intn(maxIncoming+1) + Settings.minIncomingPosts

		if numberOfRequests <= routinesLeft {
			simulatedRequestsChan <- numberOfRequests
			log("simulating incoming posts", "count", numberOfRequests, "min", Settings.minIncomingPosts, "max", Settings.maxIncomingPosts)
			for i := 1; i <= numberOfRequests; i++ {
				go simulatePostToChat(channelId, msUserId, message, i, numberOfRequests)
			}
		} else {
			log("skipping incoming simulation as numberOfRequests is more than the routines left", "left", routinesLeft, "numberOfRequests", numberOfRequests)
		}
	} else {
		log("skipping incoming simulation as there are no more routines left", "left", routinesLeft, "sim", simulatedRequests, "max", maxRoutines)
	}
}
