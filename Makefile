TAG = $(shell git rev-parse --short HEAD)
IMAGE = valshatravenko/protohackers:$(TAG)
TARGET_BINARY ?= smoke_test

.PHONY: help
help:
	@echo 'Usage: (use TARGET_BINARY to specify the challenge)'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

out:
	mkdir out

.PHONY: smoke
smoke: out
	CGO_ENABLED=0 go build -o out/smoke_test ./cmd/smoke_test

.PHONY: run-smoke
run-smoke: smoke
	go run ./cmd/smoke_test

## run: run the target binary locally
.PHONY: run
run:
	go run ./cmd/$(TARGET_BINARY)

.PHONY: prime
prime: out
	CGO_ENABLED=0 go build -o out/prime_time ./cmd/prime_time

.PHONY: run-smoke
run-prime: prime
	go run ./cmd/prime_time

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
