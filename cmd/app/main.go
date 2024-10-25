//go:build darwin
// +build darwin

package main

import (
	"github.com/progrium/darwinkit/macos"

	"github.com/bububa/osssync/internal/app"
	"github.com/bububa/osssync/internal/service"
)

func main() {
	macos.RunApp(app.Launch)
	service.Close()
}
