# GO_BUILD_ARGS should be set when running 'go build' or 'go install'.
BUILD_DIR = $(PWD)/bin
GO_BUILD_ARGS = \
  -gcflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -asmflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -tags=netgo,osusergo \
  -ldflags " \
    -s \
    -w \
    -extldflags=-static \
  " \

# Always use Go modules
export GO111MODULE = on

.PHONY: build
build:
	CGO_ENABLED=0 mkdir -p $(BUILD_DIR) && go build $(GO_BUILD_ARGS) -o $(BUILD_DIR) ./

.PHONY: docker
docker:
	docker build -t gcr.io/lanford-io/catalog-release-bot:latest -f Dockerfile .

.PHONY: docker-push
docker-push: docker
	docker push gcr.io/lanford-io/catalog-release-bot:latest
