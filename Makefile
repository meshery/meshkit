check: error
	golangci-lint run

check-clean-cache: error
	golangci-lint cache clean

test: error
	go test ./...

errorutil:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . update --skip-dirs meshery -i ./helpers -o ./helpers

errorutil-analyze:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . analyze --skip-dirs meshery -i ./helpers -o ./helpers

build-errorutil:
	go build -o errorutil cmd/errorutil/main.go
