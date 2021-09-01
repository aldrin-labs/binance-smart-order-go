package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/logging"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"go.uber.org/zap"
	"time"
)

func GetLoggerStatsd() (interfaces.ILogger, interfaces.IStatsClient) {
	logger, _ := logging.GetZapLogger()
	logger = logger.With(zap.String("logger", "ss"))
	statsd := &MockStatsdClient{Client: nil, Log: logger}
	return logger, statsd
}

func WaitDisableSmartOrder(duration time.Duration, order *smart_order.SmartOrder) {
	time.Sleep(duration)
	order.Strategy.GetModel().Enabled = false
}