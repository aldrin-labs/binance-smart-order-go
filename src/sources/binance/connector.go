package binance

import (
	"context"
	"fmt"
	"github.com/Cryptocurrencies-AI/go-binance"
	"github.com/go-kit/kit/log"
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

	fmt.Println("here ", done, err)

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

	fmt.Println("waiting for interrupt")
	<-interrupt
	fmt.Println("canceling context")
	cancelCtx()
	fmt.Println("waiting for signal")
	<-done
	fmt.Println("exit")
	return nil
}

func ListenBinanceSpread(onMessage func(data *binance.SpreadAllEvent) error) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	binanceInstance, cancelCtx := GetBinanceClientInstance()
	kech, done, err := binanceInstance.SpreadAllWebsocket()

	fmt.Println("here ", done, err)

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

	fmt.Println("waiting for interrupt")
	<-interrupt
	fmt.Println("canceling context")
	cancelCtx()
	fmt.Println("waiting for signal")
	<-done
	fmt.Println("exit")
	return nil
}