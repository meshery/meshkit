check: error
	golangci-lint run

check-clean-cache: error
	golangci-lint cache clean

test: error
	go test ./...

error: errorutil
	./errorutil -d . update

errorutil:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . update