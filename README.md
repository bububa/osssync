# Introduction

oss-sync is a tool to sync local files to aliyun.com OSS.

# Configuration

```toml
[[Settings]]
Local = "local folders to sync"
IgnoreHiddenFiles = true # ignore local hidden files
Endpoint = "oss-cn-zhangjiakou.aliyuncs.com" # oss endpoint
Bucket = "gperf" # oss bucket name
Prefix = "sync" # oss bucket storage file prefix
AccessKeyID = "oss access key id"
AccessKeySecret = "oss access key secret"
Delete = false # delete oss files if local file deleted
```

## for linux

- ~/.config/org.musicpeace.osssync/config
- /etc/xdg/org.musicpeace.osssync/config
- /etc/org.musicpeace.osssync/config

## for Mac

~/Library/Preferences/org.musicpeace.osssync/config

# Install

## for CLI

```bash
make cli
./dist/osssync-cli sync
```

## for GUI

````bash
make darwin
open ./dist/osssync.app

# Make
```bash
make bundle
````
