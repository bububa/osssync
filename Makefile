PROJECT_NAME=osssync
GIT_TAG = $(shell git tag | grep ^v | sort -V | tail -n 1)
GIT_REVISION = $(shell git rev-parse --short HEAD)
GIT_SUMMARY = $(shell git describe --tags --dirty --always)
GIT_CONFIG_PATH=github.com/bububa/osssync/internal/config
ENTRY_POINT = ./cmd/app
DIST_PATH=./dist
LDFLAGS = -X $(GIT_CONFIG_PATH).GitTag=$(GIT_TAG) -X $(GIT_CONFIG_PATH).GitRevision=$(GIT_REVISION) -X $(GIT_CONFIG_PATH).GitSummary=$(GIT_SUMMARY) -s -w -extldflags "-static"

.PHONY : all

all: app

app:
ifeq (,$(wildcard $(DIST_PATH)/$(PROJECT_PATH)))
	rm -rf $(DIST_PATH)/$(PROJECT_NAME)
endif
	go build -v -ldflags "$(LDFLAGS)" -o $(DIST_PATH)/$(PROJECT_NAME) $(ENTRY_POINT)

clean:
	rm -rf $(DIST_PATH)/$(PROJECT_NAME)
