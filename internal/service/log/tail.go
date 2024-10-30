package log

import (
	"github.com/grafana/tail"
)

type Tail struct {
	cfg  *tail.Config
	path string
}

func NewTail(path string, cfg *tail.Config) *Tail {
	return &Tail{path: path, cfg: cfg}
}

func (t *Tail) SetLocation(offset int64) {
	if t.cfg.Location == nil {
		t.cfg.Location = new(tail.SeekInfo)
	}
	t.cfg.Location.Offset = offset
}

func (t *Tail) Reset() {
	t.cfg.Location = nil
}

var tailer *Tail

func TailStart() (*tail.Tail, error) {
	return tail.TailFile(tailer.path, *tailer.cfg)
}

func TailStop(t *tail.Tail) error {
	if offset, err := t.Tell(); err == nil {
		tailer.SetLocation(offset)
	}
	return t.Stop()
}

func TailCleanup(t *tail.Tail) {
	tailer.Reset()
	t.Cleanup()
}
