package binance

import (
	"context"
	"github.com/Cryptocurrencies-AI/go-binance"
	"github.com/go-kit/kit/log"
	loggly_client "gitlab.com/crypto_project/core/strategy_service/src/sources/loggy"
	"os"
	"os/signal"
)

func GetBinanceClientInstance() (binance.Binance, context.CancelFunc) {
	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "time", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	ctx, cancelCtx := context.WithCancel(context.Background())
	// use second return value for cancelling request when shutting down the app

	binanceService := binance.NewAPIService(
		"https://www.binance.com",
		"",
		nil,
		logger,
		ctx,
	)
	b := binance.NewBinance(binanceService)
	return b, cancelCtx
}

func ListenBinanceMarkPrice(onMessage func(data *binance.MarkPriceAllStrEvent) error) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	binance, cancelCtx := GetBinanceClientInstance()
	kech, done, err := binance.MarkPriceAllStrWebsocket()

	loggly_client.GetInstance().Info("here ", done, err)

	if err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case ke := <-kech:
				_ = onMessage(ke)
			case <-done:
				break
			}
		}
	}()

	loggly_client.GetInstance().Info("waiting for interrupt")
	<-interrupt
	loggly_client.GetInstance().Info("canceling context")
	cancelCtx()
	loggly_client.GetInstance().Info("waiting for signal")
	<-done
	loggly_client.GetInstance().Info("exit")
	return nil
}

func ListenBinanceSpread(onMessage func(data *binance.SpreadAllEvent) error) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	binanceInstance, cancelCtx := GetBinanceClientInstance()
	kech, done, err := binanceInstance.SpreadAllWebsocket()

	loggly_client.GetInstance().Info("here ", done, err)

	if err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case ke := <-kech:
				//loggly_client.GetInstance().Infof("%#v\n", ke)
				_ = onMessage(ke)
			case <-done:
				break
			}
		}
	}()

	loggly_client.GetInstance().Info("waiting for interrupt")
	<-interrupt
	loggly_client.GetInstance().Info("canceling context")
	cancelCtx()
	loggly_client.GetInstance().Info("waiting for signal")
	<-done
	loggly_client.GetInstance().Info("exit")
	return nil
}
