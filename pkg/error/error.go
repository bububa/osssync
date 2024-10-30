package error

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"

	"github.com/bububa/osssync/pkg/fs/local"
)

type Error struct {
	cause error
	files []*local.FileInfo
	op    fsnotify.Op
}

func NewError(op fsnotify.Op, cause error, files ...*local.FileInfo) Error {
	return Error{
		op:    op,
		cause: cause,
		files: files,
	}
}

func (e Error) Error() string {
	if l := len(e.files); l > 0 {
		names := make([]string, 0, l)
		for _, f := range e.files {
			names = append(names, f.Path())
		}
		return fmt.Sprintf("[%s] %s, %s", e.op.String(), strings.Join(names, ", "), e.cause.Error())
	}
	return fmt.Sprintf("[%s], %s", e.op.String(), e.cause.Error())
}

func (e Error) Cause() error {
	return e.cause
}
