package observability

import (
	"go.uber.org/zap"
)

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

type ZapLogger struct {
	logger *zap.Logger
}

func NewLogger() Logger {
	config := zap.NewProductionConfig()
	config.Encoding = "json"
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, _ := config.Build()
	return &ZapLogger{logger: logger}
}

func (zl *ZapLogger) Info(msg string, keysAndValues ...interface{}) {
	zl.logger.Sugar().Infow(msg, keysAndValues...)
}

func (zl *ZapLogger) Error(msg string, keysAndValues ...interface{}) {
	zl.logger.Sugar().Errorw(msg, keysAndValues...)
}

func (zl *ZapLogger) Warn(msg string, keysAndValues ...interface{}) {
	zl.logger.Sugar().Warnw(msg, keysAndValues...)
}

func (zl *ZapLogger) Sync() error {
	return zl.logger.Sync()
}
