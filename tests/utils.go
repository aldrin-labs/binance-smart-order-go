package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/logging"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"go.uber.org/zap"
)

func GetLoggerStatsd() (interfaces.ILogger, interfaces.IStatsClient) {
	logger, _ := logging.GetZapLogger()
	logger = logger.With(zap.String("logger", "ss"))
	statsd := &MockStatsdClient{Client: nil, Log: logger}
	return logger, statsd
}