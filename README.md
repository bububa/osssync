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

- ~/.config/com.thepeppersstudio.osssync/config
- /etc/xdg/com.thepeppersstudio.osssync/config
- /etc/com.thepeppersstudio.osssync/config

## for Mac

~/Library/Preferences/com.thepeppersstudio.osssync/config

# Install

## for linux

```bash
make cli
./dist/oss-sync-cli sync
```

## for MacOS

```bash
make darwin
open ./dist/oss-sync.app
```
