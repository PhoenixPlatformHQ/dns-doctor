BINARY    := dns-doctor
MODULE    := github.com/phoenix-platform/dns-doctor
CMD       := ./cmd/dns-doctor
VERSION   := 0.1.0
LDFLAGS   := -ldflags "-X main.version=$(VERSION)"

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
DIST      := dist

.PHONY: build build-all test lint clean

## build: compile for the current platform
build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

## build-all: cross-compile for all target platforms
build-all:
	@mkdir -p $(DIST)
	@$(foreach PLATFORM,$(PLATFORMS), \
		$(eval GOOS=$(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval GOARCH=$(word 2,$(subst /, ,$(PLATFORM)))) \
		$(eval EXT=$(if $(filter windows,$(GOOS)),.exe,)) \
		echo "Building $(GOOS)/$(GOARCH)..."; \
		GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) \
			-o $(DIST)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT) $(CMD); \
	)

## test: run all unit tests
test:
	go test ./... -v -count=1

## lint: run golangci-lint (requires golangci-lint to be installed)
lint:
	@which golangci-lint > /dev/null 2>&1 || \
		(echo "golangci-lint not found — skipping. Install via https://golangci-lint.run/usage/install/"; exit 0)
	golangci-lint run ./...

## clean: remove build artefacts
clean:
	@rm -f $(BINARY)
	@rm -rf $(DIST)
