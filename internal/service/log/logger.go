package log

import (
	"github.com/rs/zerolog"
)

var logger *zerolog.Logger

func Logger() *zerolog.Logger {
	return logger
}
