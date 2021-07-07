package interfaces

import (
	"go.uber.org/zap"
)

type ILogger interface {
	Info(s string, field ...zap.Field)
	Warn(s string, field ...zap.Field)
	Fatal(s string, field ...zap.Field)
	Error(s string, field ...zap.Field)
	Debug(s string, field ...zap.Field)
}