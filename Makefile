check:
	golangci-lint run

check-clean-cache:
	golangci-lint cache clean

test:
	go test ./...

errorutil:
	go build -o errorutil cmd/errorutil/main.go