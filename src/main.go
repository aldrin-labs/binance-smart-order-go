package main

import (
    "github.com/joho/godotenv"
    "gitlab.com/crypto_project/core/strategy_service/src/server"
    "gitlab.com/crypto_project/core/strategy_service/src/service"
    "log"
    "sync"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }
    var wg sync.WaitGroup
    //TODO: init top-level context
    //notif := filtering.NewNotifier()
    //log.Println(notif)
    //sub := mongodb.NewSubscription(notif, "ccai-dev", "notifications")
    //go sub.RunDataPull()
    //log.Println(err)
    //redisSub := redis.NewSubscription(notif)
    //go redisSub.RunDataPull()
    wg.Add(1)
    go server.RunServer(&wg)
    wg.Add(1)
    go service.InitSingleton(&wg)
    wg.Wait()
}
