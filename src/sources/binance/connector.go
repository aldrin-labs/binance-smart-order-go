package binance

import (
	"context"
	"fmt"
	"github.com/Cryptocurrencies-AI/go-binance"
	"go.uber.org/zap"
	"os"
	"os/signal"
)

func GetBinanceClientInstance() (binance.Binance, context.CancelFunc) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	// use second return value for cancelling request when shutting down the app

	binanceService := binance.NewAPIService(
		"https://www.binance.com",
		"",
		nil,
		nil,
		ctx,
	)
	b := binance.NewBinance(binanceService)
	return b, cancelCtx
}

func ListenBinanceMarkPrice(onMessage func(data *binance.RawEvent) error) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	binance, cancelCtx := GetBinanceClientInstance()
	kechSpot, doneSpot, err := binance.SpotAllMarketMiniTickersStreamWebsocket()
	if err != nil {
		return fmt.Errorf("listen spot: %v", err)
	}
	kechFutures, doneFutures, err := binance.MarkPriceStreamAllMarketWebsocket()
	if err != nil {
		return fmt.Errorf("listen futures: %v", err)
	}

	go func() {
		for {
			select {
			case e := <-kechFutures:
				onMessage(e)
			case e := <-kechSpot:
				onMessage(e)
			case <-doneSpot:
				break
			case <-doneFutures:
				break
			}
		}
	}()

	log.Info("waiting for interrupt")
	<-interrupt
	log.Info("canceling context")
	cancelCtx()
	log.Info("waiting for signal")
	<-doneSpot
	<-doneFutures
	log.Info("exit")
	return nil
}

func ListenBinanceSpread(onMessage func(data *binance.SpreadAllEvent) error) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	binanceInstance, cancelCtx := GetBinanceClientInstance()
	kech, done, err := binanceInstance.SpreadAllWebsocket()

	log.Info("here",
		zap.String("done", fmt.Sprintf("%v", done)),
		zap.Error(err),
	)

	if err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case ke := <-kech:
				//fmt.Printf("%#v\n", ke)
				_ = onMessage(ke)
			case <-done:
				break
			}
		}
	}()

	log.Info("waiting for interrupt")
	<-interrupt
	log.Info("canceling context")
	cancelCtx()
	log.Info("waiting for signal")
	<-done
	log.Info("exit")
	return nil
}
