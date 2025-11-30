TAG = $(shell git rev-parse --short HEAD)
IMAGE = valshatravenko/protohackers:$(TAG)
TARGET_BINARY ?= smoke_test

.PHONY: help
help:
	@echo 'Usage: (use TARGET_BINARY to specify the challenge)'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

out:
	mkdir out

.PHONY: build
build: out
	CGO_ENABLED=0 go build -o out/$(TARGET_BINARY) ./cmd/$(TARGET_BINARY)

.PHONY: build
build-linux: out
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o out/$(TARGET_BINARY) ./cmd/$(TARGET_BINARY)

## run: run the target binary locally
.PHONY: run
run:
	go run ./cmd/$(TARGET_BINARY)

## test: execute tests for the target binary
.PHONY: test
test:
	go test -v ./cmd/$(TARGET_BINARY)/*go

## bench: execute benchmarks for the target binary
.PHONY: bench
bench:
	go test -v -run='^$' -bench='^Bench.*$' ./cmd/$(TARGET_BINARY)/*go

## docker: build the target Docker image
.PHONY: docker
docker:
	docker build --platform linux/amd64 -t $(IMAGE) --build-arg TARGET_BINARY=$(TARGET_BINARY) .

.PHONY: docker-run
docker-run:
	docker run -it $(IMAGE)

## deploy: deploy to fly.io(make sure to set $FLY_APP_NAME)
.PHONY: deploy
deploy:
	@echo Deploying $(TARGET_BINARY)
	fly deploy -a $(FLY_APP_NAME) --build-arg TARGET_BINARY=$(TARGET_BINARY)

## upload-dev
.PHONY: upload-dev
upload-dev: build-linux
	@echo Uploading $(TARGET_BINARY) to $(DEV_VM_USER)@$(DEV_VM_IP)
	scp out/$(TARGET_BINARY) $(DEV_VM_USER)@$(DEV_VM_IP):

## connect-dev
.PHONY: connect-dev
connect-dev:
	ssh $(DEV_VM_USER)@$(DEV_VM_IP)
