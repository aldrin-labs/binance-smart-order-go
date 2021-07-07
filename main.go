package main

import (
	"context"
	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/server"
	"gitlab.com/crypto_project/core/strategy_service/src/service"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Print("Error loading .env file") // TODO (Alisher): mb panic with no env?
	}
}

func main() {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT) // k8s sends SIGTERM and waits
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("Brute shutdown.")
		os.Exit(0)
	}()

	var wg sync.WaitGroup // TODO: init top-level context
	wg.Add(1)
	go server.RunServer(&wg)
	wg.Add(1)
	isLocalBuild := os.Getenv("LOCAL") == "true"
	go service.GetStrategyService().Init(&wg, isLocalBuild)
	wg.Wait()
}
