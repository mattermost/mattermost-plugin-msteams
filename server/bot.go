package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/infracloudio/msbotbuilder-go/core"
	"github.com/infracloudio/msbotbuilder-go/core/activity"
	"github.com/infracloudio/msbotbuilder-go/schema"
)

var customHandler = activity.HandlerFuncs{
	OnMessageFunc: func(turn *activity.TurnContext) (schema.Activity, error) {
		return turn.SendActivity(activity.MsgOptionText("Echo: " + turn.Activity.Text))
	},
}

// HTTPHandler handles the HTTP requests from then connector service
type HTTPHandler struct {
	core.Adapter
	p *Plugin
}

func (ht *HTTPHandler) processMessage(w http.ResponseWriter, req *http.Request) {
	ht.p.API.LogError("PROCCESING MESSAGE")
	ctx := context.Background()
	activity := schema.Activity{}
	// Parse request body
	err := json.NewDecoder(req.Body).Decode(&activity)
	if err != nil {
		fmt.Println("Failed to parse request.", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = ht.Adapter.ProcessActivity(ctx, activity, customHandler)
	if err != nil {
		fmt.Println("Failed to process request", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println(activity)
	fmt.Println("Request processed successfully.")
}
