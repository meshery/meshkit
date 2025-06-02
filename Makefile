include build/Makefile.core.mk
include build/Makefile.show-help.mk

## Run suite of Golang lint checks
check:
	golangci-lint run -c .golangci.yml -v ./...

## Run Golang tests
test:
	go test --short ./... -race -coverprofile=coverage.txt -covermode=atomic

## Clean up Golang packages. Print diff.
tidy:
	go mod tidy
	git diff --exit-code go.mod go.sum

## Run Meshery Error Code Utility. Generate error codes.
errorutil:
	go run github.com/meshery/meshkit/cmd/errorutil -d . update --skip-dirs meshery -i ./helpers -o ./helpers

## Run Meshery Error Code Utility. Analyze only.
errorutil-analyze:
	go run github.com/meshery/meshkit/cmd/errorutil -d . analyze --skip-dirs meshery -i ./helpers -o ./helpers

## Build the Meshery Error Code Utility. 
build-errorutil:
	go build -o errorutil cmd/errorutil/main.go
