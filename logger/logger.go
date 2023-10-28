package logger

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideLogger() *zap.SugaredLogger {
	l, _ := zap.NewProduction()
	slogger := l.Sugar()
	return slogger
}

var Module = fx.Options(
	fx.Provide(ProvideLogger),
)
