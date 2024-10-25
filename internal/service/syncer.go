package service

import (
	"github.com/bububa/osssync/internal/service/sync"
)

var syncer = sync.NewSyncer()

func Syncer() *sync.Syncer {
	return syncer
}
