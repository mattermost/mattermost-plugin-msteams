module github.com/mattermost/mattermost-plugin-matterbridge

go 1.16

replace github.com/42wim/matterbridge => ../matterbridge

require (
	github.com/42wim/matterbridge v1.25.2
	github.com/davecgh/go-spew v1.1.1
	github.com/infracloudio/msbotbuilder-go v0.2.5
	github.com/matterbridge/logrus-prefixed-formatter v0.5.3-0.20200523233437-d971309a77ba
	github.com/mattermost/mattermost-server/v6 v6.7.2
	github.com/mattn/godown v0.0.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.1
	github.com/yaegashi/msgraph.go v0.1.4
	golang.org/x/oauth2 v0.1.0
)
