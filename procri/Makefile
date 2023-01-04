NAME ?= $(shell basename $(TOP_DIR))

REGISTRY_REPO ?= 689494258501.dkr.ecr.us-east-1.amazonaws.com/elotl/$(NAME)

DKR=docker
GIT_VERSION=$(shell git describe --dirty)
CURRENT_TIME=$(shell date +%Y%m%d%H%M%S)
IMAGE_TAG=$(GIT_VERSION)
ifneq ($(findstring -,$(GIT_VERSION)),)
IMAGE_DEV_OR_LATEST=dev
else
IMAGE_DEV_OR_LATEST=latest
endif

LD_VERSION_FLAGS=-X main.BuildVersion=$(GIT_VERSION) -X main.BuildTime=$(CURRENT_TIME)
LDFLAGS=-ldflags "$(LD_VERSION_FLAGS)"

BINARIES=$(NAME)-darwin-amd64 $(NAME)-darwin-arm64

TOP_DIR=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))
CMD_SRC=$(shell find $(TOP_DIR)cmd -type f -name '*.go')
PKG_SRC=$(shell find $(TOP_DIR)pkg -type f -name '*.go')

all: $(BINARIES) go.mod go.sum

$(NAME)-darwin-amd64: $(CMD_SRC) $(PKG_SRC)
	GOARCH=amd64 GOOS=darwin go build $(LDFLAGS) -o $(TOP_DIR)$@ cmd/$(NAME)/main.go

$(NAME)-darwin-arm64: $(CMD_SRC) $(PKG_SRC)
	GOARCH=arm64 GOOS=darwin go build $(LDFLAGS) -o $(TOP_DIR)$@ cmd/$(NAME)/main.go

test: $(BINARIES)
	go test ./... -v

img: $(BINARIES)
	@echo "Checking if IMAGE_TAG is set" && test -n "$(IMAGE_TAG)"
	$(DKR) build -t $(REGISTRY_REPO):$(IMAGE_TAG) \
		-t $(REGISTRY_REPO):$(IMAGE_DEV_OR_LATEST) .

login-img:
	@echo "Checking if REGISTRY_USER is set" && test -n "$(REGISTRY_USER)"
	@echo "Checking if REGISTRY_PASSWORD is set" && test -n "$(REGISTRY_PASSWORD)"
	@$(DKR) login -u "$(REGISTRY_USER)" -p "$(REGISTRY_PASSWORD)" "$(REGISTRY_SERVER)"

push-img: img
	@echo "Checking if IMAGE_TAG is set" && test -n "$(IMAGE_TAG)"
	$(DKR) push $(REGISTRY_REPO):$(IMAGE_TAG)
	$(DKR) push $(REGISTRY_REPO):$(IMAGE_DEV_OR_LATEST)

clean:
	rm -f $(BINARIES)

.PHONY: all clean push-img login-img img test $(NAME)-darwin-amd64 $(NAME)-darwin-arm64
