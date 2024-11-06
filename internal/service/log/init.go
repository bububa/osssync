package log

import (
	"log"
	"path/filepath"

	"github.com/adrg/xdg"
	logPkg "github.com/bububa/osssync/pkg/log"
	"github.com/grafana/tail"

	"github.com/bububa/osssync/internal/config"
)

func init() {
	logPath, err := xdg.DataFile(filepath.Join(config.AppIdentity, config.AppLog))
	if err != nil {
		log.Fatalln(err)
	}
	logger = logPkg.NewLogger(logPath)
	tailer = NewTail(logPath, &tail.Config{Follow: true, Logger: tail.DiscardingLogger})
}
