module github.com/bububa/osssync

go 1.23.1

replace github.com/aliyun/ossutil => github.com/bububa/ossutil v1.0.0

// replace github.com/aliyun/ossutil => ../ossutil

require (
	github.com/aliyun/ossutil v1.7.19
	github.com/jinzhu/configor v1.2.2
	github.com/muesli/go-app-paths v0.2.2
	github.com/progrium/darwinkit v0.5.0
	github.com/urfave/cli/v2 v2.27.5
	go.uber.org/atomic v1.11.0
	gopkg.in/fsnotify.v1 v1.4.7
)

require (
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/aliyun/aliyun-oss-go-sdk v3.0.2+incompatible // indirect
	github.com/alyu/configparser v0.0.0-20191103060215-744e9a66e7bc // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/droundy/goopt v0.0.0-20220217183150-48d6390ad4d1 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/term v0.25.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
