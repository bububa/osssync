package log

import (
	"log"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/bububa/osssync/pkg"
	logPkg "github.com/bububa/osssync/pkg/log"
	"github.com/grafana/tail"

	"github.com/bububa/osssync/internal/config"
)

func init() {
	logPath, err := xdg.DataFile(filepath.Join(pkg.AppIdentity, config.AppLog))
	if err != nil {
		log.Fatalln(err)
	}
	logger = logPkg.NewLogger(logPath)
	tailer = NewTail(logPath, &tail.Config{Follow: true, Logger: tail.DiscardingLogger})
}
