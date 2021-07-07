package interfaces

import (
	"go.uber.org/zap"
)

type ILogger interface {
	Info(s string, fields ...zap.Field)
	Warn(s string, fields ...zap.Field)
	Fatal(s string, fields ...zap.Field)
	Error(s string, fields ...zap.Field)
	Debug(s string, fields ...zap.Field)
	Panic(s string, fields ...zap.Field)
}