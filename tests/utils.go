package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"go.uber.org/zap"
	"os"
)

func GetLoggerStatsd() (*zap.Logger, interfaces.IStatsClient) {
	var logger *zap.Logger
	if os.Getenv("LOCAL") == "true" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction() // TODO(khassanov): handle the error
	}
	logger = logger.With(zap.String("logger", "ss"))
	statsd := &MockStatsdClient{Client: nil, Log: logger}
	return logger, statsd
}