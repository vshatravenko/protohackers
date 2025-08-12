TAG = $(shell git rev-parse --short HEAD)
IMAGE = valshatravenko/protohackers:$(TAG)

out:
	mkdir out

.PHONY: smoke
build-smoke: out
	CGO_ENABLED=0 go build -o out/smoke ./cmd/smoke_test

.PHONY: run-smoke
run-smoke: smoke
	go run ./cmd/smoke_test

.PHONY: docker
docker:
	docker build --platform linux/amd64 -t $(IMAGE) .

.PHONY: docker-run
docker-run:
	docker run -it $(IMAGE)

.PHONY: deploy
deploy:
	fly deploy -a $(FLY_APP_NAME)
