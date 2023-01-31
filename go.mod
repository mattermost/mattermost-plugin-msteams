module github.com/mattermost/mattermost-plugin-matterbridge

go 1.16

replace github.com/42wim/matterbridge => ../matterbridge

require (
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/klauspost/compress v1.15.8 // indirect
	github.com/mattermost/mattermost-server/v6 v6.7.2
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/mattn/godown v0.0.1
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/stretchr/testify v1.8.1
	github.com/yaegashi/msgraph.go v0.1.4
	golang.org/x/crypto v0.0.0-20221012134737-56aed061732a // indirect
	golang.org/x/oauth2 v0.1.0
	google.golang.org/protobuf v1.28.1 // indirect
)
