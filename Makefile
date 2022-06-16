all: build

build: fmt vet
	go test $(shell go build ./...)

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

test:
	go test $(shell go list ./...) -coverprofile cover.out
