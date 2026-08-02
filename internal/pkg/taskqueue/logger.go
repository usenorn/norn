package taskqueue

import (
	"fmt"
	"log/slog"
)

type serverLogger struct {
	logger *slog.Logger
}

func (l serverLogger) Debug(args ...any) {
	l.logger.Debug(fmt.Sprint(args...))
}

func (l serverLogger) Info(args ...any) {
	l.logger.Info(fmt.Sprint(args...))
}

func (l serverLogger) Warn(args ...any) {
	l.logger.Warn(fmt.Sprint(args...))
}

func (l serverLogger) Error(args ...any) {
	l.logger.Error(fmt.Sprint(args...))
}

func (l serverLogger) Fatal(args ...any) {
	l.logger.Error(fmt.Sprint(args...))
}
