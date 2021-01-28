package main

import (
	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/server"
	"gitlab.com/crypto_project/core/strategy_service/src/service"
	"log"
	"os"
	"sync"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Print("Error loading .env file") // TODO (Alisher): mb panic with no env?
	}
}

func main() {
	var wg sync.WaitGroup // TODO: init top-level context
	wg.Add(1)
	go server.RunServer(&wg)
	wg.Add(1)
	isLocalBuild := os.Getenv("LOCAL") == "true"
	go service.GetStrategyService().Init(&wg, isLocalBuild)
	wg.Wait()
}
