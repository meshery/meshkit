check:
	golangci-lint run

check-clean-cache:
	golangci-lint cache clean

test:
	go test ./...

errorutil:
	go run github.com/meshery/meshkit/cmd/errorutil -d . update --skip-dirs meshery -i ./helpers -o ./helpers

errorutil-analyze:
	go run github.com/meshery/meshkit/cmd/errorutil -d . analyze --skip-dirs meshery -i ./helpers -o ./helpers

build-errorutil:
	go build -o errorutil cmd/errorutil/main.go
