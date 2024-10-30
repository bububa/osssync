package log

import (
	"log"
	"os"
	"path/filepath"

	logPkg "github.com/bububa/osssync/pkg/log"
	"github.com/grafana/tail"
	gap "github.com/muesli/go-app-paths"

	"github.com/bububa/osssync/internal/config"
)

func init() {
	scope := gap.NewScope(gap.User, config.AppIdentity)
	var logPath string
	logPath, err := scope.LogPath(config.AppLog)
	if err != nil {
		log.Fatalln(err)
	}
	dir := filepath.Dir(logPath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			log.Fatalln(err)
		}
	}
	logger = logPkg.NewLogger(logPath)
	tailer = NewTail(logPath, &tail.Config{Follow: true, Logger: tail.DiscardingLogger})
}
