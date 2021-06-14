check: error
	golangci-lint run

check-clean-cache: error
	golangci-lint cache clean

test: error
	go test ./...

error: errorutil
	./errorutil -d . update

errorutil:
	go build -o errorutil cmd/errorutil/main.go