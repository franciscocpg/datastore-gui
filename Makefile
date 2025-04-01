export GOBIN := $(PWD)/$(BIN_FOLDER)
SHA_COMMIT := $(shell git rev-parse --short HEAD)
IMAGE_NAME := franciscocpg/datastore-gui:$(SHA_COMMIT)

# It installs gow: https://github.com/mitranim/gow
gow := $(GOBIN)/gow
$(gow):
	@go install -v github.com/mitranim/gow@87df6e4

# It runs the server locally in watch mode
.PHONY: dev
dev: $(gow)
	@$(gow) -c -v run main.go -port=$(PORT) -projectID=$(PROJECT_ID) -dsHost=$(DATASTORE_EMULATOR_HOST) -entities=$(ENTITIES)

# It builds the docker image
.PHONY: docker-build
docker-build:
	@docker build -t $(IMAGE_NAME) . --platform=linux

# It pushes the docker image to the registry
.PHONY: docker-push
docker-push:
	@docker push $(IMAGE_NAME)
