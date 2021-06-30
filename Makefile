check: error
	golangci-lint run

check-clean-cache: error
	golangci-lint cache clean

test: error
	go test ./...

errorutil:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . update

errorutil-analyze:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . analyze

build-errorutil:
	go build -o errorutil cmd/errorutil/main.go