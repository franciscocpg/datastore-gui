export GOBIN := $(PWD)/$(BIN_FOLDER)
SHA_COMMIT := $(shell git rev-parse --short HEAD)
IMAGE_NAME := franciscocpg/datastore-gui:$(SHA_COMMIT)

# It installs gow: https://github.com/mitranim/gow
gow := $(GOBIN)/gow
$(gow):
	@go install -v github.com/mitranim/gow@87df6e4

# It builds the client
.PHONY: build-client
build-client:
	@cd client && yarn install && node build.ts

# It runs the server locally in watch mode
.PHONY: dev
dev: $(gow) build-client
	@$(gow) -c -v run main.go -port=$(PORT) -projectID=$(PROJECT_ID) -dsHost=$(DATASTORE_EMULATOR_HOST) -entities=$(ENTITIES)

# It enables multi-platform buildx
.PHONY: enable-multi-platform
enable-multi-platform:
	@docker buildx create --name container-builder --driver docker-container --bootstrap --use

# It builds and push the docker image for multi-platform
# You need to run "make enable-multi-platform" at least once for the docker buildx to work with multi platform:
.PHONY: docker-build-push-multi-platform
docker-build-push-multi-platform:
	@docker buildx build -t $(IMAGE_NAME) . --platform linux/amd64,linux/arm64 --push
