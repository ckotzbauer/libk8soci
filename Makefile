TEMPDIR = ./.tmp
LINTCMD = $(TEMPDIR)/golangci-lint run --timeout 5m
GOSECCMD = $(TEMPDIR)/gosec ./...

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

lint:
	$(LINTCMD)

lintsec:
	$(GOSECCMD)

$(TEMPDIR):
	mkdir -p $(TEMPDIR)

.PHONY: bootstrap-tools
bootstrap-tools: $(TEMPDIR)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TEMPDIR)/ v2.0.2
	curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $(TEMPDIR)/ v2.22.3
