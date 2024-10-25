CLI_NAME=oss-sync-cli
APP_NAME=oss-sync
GIT_TAG = $(shell git tag | grep ^v | sort -V | tail -n 1)
GIT_REVISION = $(shell git rev-parse --short HEAD)
GIT_SUMMARY = $(shell git describe --tags --dirty --always)
GIT_CONFIG_PATH=github.com/bububa/osssync/internal/config
CLI_ENTRY_POINT = ./cmd/cli
APP_ENTRY_POINT = ./cmd/app
DIST_PATH=./dist
CLI_LDFLAGS = -X $(GIT_CONFIG_PATH).GitTag=$(GIT_TAG) -X $(GIT_CONFIG_PATH).GitRevision=$(GIT_REVISION) -X $(GIT_CONFIG_PATH).GitSummary=$(GIT_SUMMARY) -s -w -extldflags "-static"
DARWIN_LDFLAGS = -X $(GIT_CONFIG_PATH).GitTag=$(GIT_TAG) -X $(GIT_CONFIG_PATH).GitRevision=$(GIT_REVISION) -X $(GIT_CONFIG_PATH).GitSummary=$(GIT_SUMMARY) -s -w
GOOS ?=
ARCH ?=
CERT ?=
BUNDLED ?=

APPBUNDLE=$(APP_NAME).app
APPBUNDLECONTENTS=$(APPBUNDLE)/Contents
APPBUNDLEEXE=$(APPBUNDLECONTENTS)/MacOS
APPBUNDLERESOURCES=$(APPBUNDLECONTENTS)/Resources
APPBUNDLEICON=$(APPBUNDLECONTENTS)/Resources

ASSETS=assets

DARWINASSETS=$(ASSETS)/darwin
DARWIN_IMAGESET=$(DARWINASSETS)/imageset
DARWIN_ICONS=$(DARWINASSETS)/icons

.PHONY : all

all: cli app darwin

cli:
ifeq (,$(wildcard $(DIST_PATH)/$(CLI_NAME)))
	rm -rf $(DIST_PATH)/$(CLI_NAME)
endif
	go build -ldflags "$(CLI_LDFLAGS)" -o $(DIST_PATH)/$(CLI_NAME) $(CLI_ENTRY_POINT)

darwin_app:
ifeq (,$(wildcard $(DIST_PATH)/$(APP_NAME)))
	rm -rf $(DIST_PATH)/$(APP_NAME)
endif
ifeq ($(strip $(BUILDED)),)
	CGO_ENABLED=1 GOOS=darwin GOARCH=$(ARCH) go build -ldflags "$(DARWIN_LDFLAGS)" -o $(DIST_PATH)/$(APP_NAME) $(APP_ENTRY_POINT)
else
	CGO_ENABLED=1 GOOS=darwin GOARCH=$(ARCH) go build -ldflags "$(DARWIN_LDFLAGS) -X $(GIT_CONFIG_PATH).Bundled=true" -o $(DIST_PATH)/$(APP_NAME) $(APP_ENTRY_POINT)
endif

darwin:
	make darwin_app ARCH=$(ARCH) BUNDLED=true
	make imageassets
	rm -rf $(DIST_PATH)/$(APPBUNDLE)
	mkdir $(DIST_PATH)/$(APPBUNDLE)
	#xcrun lipo -o $(DIST_PATH)/$(APPBUNDLE)/$(APP_NAME) -create $(DIST_PATH)/$(APP_NAME)
	mkdir $(DIST_PATH)/$(APPBUNDLECONTENTS)
	mkdir -p $(DIST_PATH)/$(APPBUNDLEEXE)
	mkdir -p $(DIST_PATH)/$(APPBUNDLERESOURCES)
	mv $(DIST_PATH)/$(APP_NAME) $(DIST_PATH)/$(APPBUNDLEEXE)/$(APP_NAME)
	cp $(DARWINASSETS)/PkgInfo $(DIST_PATH)/$(APPBUNDLECONTENTS)/
	cp $(DARWINASSETS)/Info.plist $(DIST_PATH)/$(APPBUNDLECONTENTS)/
	/Applications/Xcode.app/Contents/Developer/usr/bin/actool --compile $(DIST_PATH)/$(APPBUNDLERESOURCES) --platform macosx --minimum-deployment-target 12.0 --app-icon AppIcon --output-partial-info-plist $(DARWINASSETS)/assets.plist $(DARWINASSETS)/Resources/Assets.xcassets
	# /usr/libexec/PlistBuddy -x -c "Merge $(DARWINASSETS)/assets.plist" $(DIST_PATH)/$(APPBUNDLECONTENTS)/Info.plist
	make icns
	mv $(DARWINASSETS)/Resources/AppIcon.icns $(DIST_PATH)/$(APPBUNDLERESOURCES)/icon.icns

# for self-signed certificate.
# Open Keychain Access.
# Choose Keychain Access > Certificate Assistant > Create Certificate ...
# Enter a name
# Set 'Certificate Type' to 'Code Signing'
codesign:
ifeq ($(strip $(CERT)),)
	$(error "missign cert name")
endif
	xattr -cr $(DIST_PATH)/$(APPBUNDLE)
	codesign -fs $(CERT) --deep --force --options runtime --entitlements $(DARWINASSETS)/Entitlements.plist $(DIST_PATH)/$(APPBUNDLE)


appicon:
	rm -rf $(DARWINASSETS)/AppIcon.appiconset
	mkdir $(DARWINASSETS)/AppIcon.appiconset
	sips -z 16 16     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/16.png
	sips -z 32 32     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/32.png
	sips -z 64 64     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/64.png
	sips -z 128 128     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/128.png
	sips -z 256 256     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/256.png
	sips -z 512 512     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/512.png
	sips -z 1024 1024     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.appiconset/1024.png
	cp $(DARWINASSETS)/AppIconContents.json $(DARWINASSETS)/AppIcon.appiconset/Contents.json
	mv $(DARWINASSETS)/AppIcon.appiconset $(DARWIN_IMAGESET)/

icns:
	rm -rf $(DARWINASSETS)/AppIcon.iconset
	mkdir $(DARWINASSETS)/AppIcon.iconset
	sips -z 16 16     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_16x16.png
	sips -z 32 32     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_16x16@2x.png
	sips -z 32 32     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_32x32.png
	sips -z 64 64     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_32x32@2x.png
	sips -z 64 64     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_64x64.png
	sips -z 128 128     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_64x64@2x.png
	sips -z 128 128     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_128x128.png
	sips -z 256 256     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_128x128@2x.png
	sips -z 256 256     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_256x256.png
	sips -z 512 512     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_256x256@2x.png
	sips -z 512 512     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_512x526.png
	sips -z 1024 1024     $(DARWINASSETS)/AppIcon.png --out $(DARWINASSETS)/AppIcon.iconset/icon_512x526@2x.png
	iconutil -c icns $(DARWINASSETS)/AppIcon.iconset -o $(DARWINASSETS)/Resources/AppIcon.icns
	rm -rf $(DARWINASSETS)/AppIcon.iconset

imageassets:
	$(foreach file, $(wildcard $(DARWIN_ICONS)/*.png), $(call generate_icon_imageasset, $(file));)
	make appicon
	rm -rf $(DARWINASSETS)/Resources/Assets.xcassets
	cp -r $(DARWIN_IMAGESET) $(DARWINASSETS)/Resources/Assets.xcassets
	cp $(DARWINASSETS)/AssetsContents.json $(DARWINASSETS)/Resources/Assets.xcassets/Contents.json
	rm -rf $(DARWIN_IMAGESET)

clean:
	rm -rf $(DIST_PATH)/*

define generate_icon_imageasset
	$(eval file=$(1))
	echo $(file)
	$(eval filename=$(basename $(notdir $(file))))
	rm -rf $(DARWIN_IMAGESET)/$(filename)
  rm -rf $(DARWIN_IMAGESET)/$(filename).imageset
	mkdir -p $(DARWIN_IMAGESET)/$(filename)
	sips -z 19 19     $(file) --out $(DARWIN_IMAGESET)/$(filename)/$(filename)@1x.png
	sips -z 38 38     $(file) --out $(DARWIN_IMAGESET)/$(filename)/$(filename)@2x.png
	sips -z 57 57     $(file) --out $(DARWIN_IMAGESET)/$(filename)/$(filename)@3x.png
	mv $(DARWIN_IMAGESET)/$(filename) $(DARWIN_IMAGESET)/$(filename).imageset
	cp $(DARWINASSETS)/render_template_image_contents.json $(DARWIN_IMAGESET)/$(filename).imageset/Contents.json
	sed -i 's/imagename/$(filename)/g' $(DARWIN_IMAGESET)/$(filename).imageset/Contents.json 
endef
