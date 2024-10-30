package log

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(logFile string) *zerolog.Logger {
	fd := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10 << 20,
		MaxBackups: 5,
		MaxAge:     60 * 60 * 24,
		LocalTime:  true,
	}
	cw := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = fd
		w.NoColor = true
	})
	w := diode.NewWriter(cw, 1000, 10*time.Millisecond, nil)
	logger := zerolog.New(w).With().Timestamp().Logger()
	return &logger
}
