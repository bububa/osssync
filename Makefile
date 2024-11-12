CLI_NAME=osssync-cli
APP_NAME=osssync
APP_BUNDLE_ID=org.musicpeace.osssync
GIT_TAG = $(shell git tag | grep ^v | sort -V | tail -n 1)
GIT_REVISION = $(shell git rev-parse --short HEAD)
GIT_SUMMARY = $(shell git describe --tags --dirty --always)
GIT_CONFIG_PATH=github.com/bububa/osssync/pkg
CLI_ENTRY_POINT = ./cmd/cli
APP_ENTRY_POINT = ./cmd/app
DIST_PATH=./dist
CLI_LDFLAGS = -X $(GIT_CONFIG_PATH).GitTag=$(GIT_TAG) -X $(GIT_CONFIG_PATH).GitRevision=$(GIT_REVISION) -X $(GIT_CONFIG_PATH).GitSummary=$(GIT_SUMMARY) -s -w -extldflags "-static"
APP_LDFLAGS = -X $(GIT_CONFIG_PATH).GitTag=$(GIT_TAG) -X $(GIT_CONFIG_PATH).GitRevision=$(GIT_REVISION) -X $(GIT_CONFIG_PATH).GitSummary=$(GIT_SUMMARY) -X ($GIT_CONFIG_PATH).AppName=$(APP_NAME) -X $(GIT_CONFIG_PATH).AppIdentity=$(APP_BUNDLE_ID) -s -w

DARWIN_BUNDLE=$(APP_NAME).app
LINUX_BUNDLE=$(APP_NAME).tar.gz
WINDOWS_BUNDLE = $(APP_NAME).exe

ASSETS=assets

DARWINASSETS=$(ASSETS)/darwin

.PHONY : all bundle

all: cli app

cli:
ifeq (,$(wildcard $(DIST_PATH)/$(CLI_NAME)))
	rm -rf $(DIST_PATH)/$(CLI_NAME)
endif
	go build -ldflags "$(CLI_LDFLAGS)" -o $(DIST_PATH)/$(CLI_NAME) $(CLI_ENTRY_POINT)

app:
	CGO_ENABLED=1 go build -v -ldflags "$(APP_LDFLAGS)" -tags no_emoji -o $(DIST_PATH)/$(APP_NAME) $(APP_ENTRY_POINT)

darwin:
	rm -rf $(DARWIN_BUNDLE) $(DIST_PATH)/
	make app
	fyne package --os darwin --id $(APP_BUNDLE_ID) --name $(APP_NAME)  --icon $(ASSETS)/AppIcon.png --exe $(DIST_PATH)/$(APP_NAME)
	mv $(DARWIN_BUNDLE) $(DIST_PATH)/
	rm -rf $(DIST_PATH)/$(APP_NAME)

linux:
	rm -rf $(LINUX_BUNDLE) $(DIST_PATH)/
	make app
	fyne package --os linux --id $(APP_BUNDLE_ID) --name $(APP_NAME)  --icon $(ASSETS)/AppIcon.png --exe $(DIST_PATH)/$(APP_NAME)
	mv $(LINUX_BUNDLE) $(DIST_PATH)/
	rm -rf $(DIST_PATH)/$(APP_NAME)

windows:
	rm -rf $(WINDOWS_BUNDLE) $(DIST_PATH)/
	make app
	fyne package --os windows --id $(APP_BUNDLE_ID) --name $(APP_NAME)  --icon $(ASSETS)/AppIcon.png --exe $(DIST_PATH)/$(APP_NAME)
	mv $(WINDOWS_BUNDLE) $(DIST_PATH)/
	rm -rf $(DIST_PATH)/$(APP_NAME)

prepare:
	go install fyne.io/fyne/v2/cmd/fyne@latest

darwin-prepare:
	xcode-select -p
	xcode-select --install
	xcodebuild -runFirstLaunch

bundle:
ifeq ($(OS), Windows_NT)
	make windows
else
ifeq ($(shell uname -s),Linux)
	make linux
endif
ifeq ($(shell uname -s),Darwin)
	echo 'darwin'
	make darwin
endif
endif


clean:
	rm -rf $(DIST_PATH)/*
