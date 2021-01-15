package loggly_client

import (
	"fmt"
	"github.com/segmentio/go-loggly"
	"log"
)

type LogglyClient struct {
	Client *loggly.Client
}

var instance *LogglyClient

func GetInstance() *LogglyClient {
	if instance == nil {
		instance = &LogglyClient{}
		instance.init()
	}
	return instance
}

func (sd *LogglyClient) init() {
	// TODO: Add LOGGLY to env & secrets
	// host := os.Getenv("LOGGLY_TOKEN")
	sd.Client = loggly.New("86c8b2ca-742d-452e-99d6-030d862d6372", "strategy-service")
	fmt.Println("Loggly client init successful")
}

func (sd *LogglyClient) Info(a ...interface{}) {
	msg := fmt.Sprint(a)
	fmt.Println(msg)
	if sd.Client != nil {
		err := sd.Client.Info(msg)
		if err != nil {
			log.Fatal(err)
		}
		sd.Client.Flush()
	}
}

func (sd *LogglyClient) Infof(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
	if sd.Client != nil {
		err := sd.Client.Info(msg)
		if err != nil {
			log.Fatal(err)
		}
		sd.Client.Flush()
	}
}

func (sd *LogglyClient) Fatal(v ...interface{}) {
	msg := fmt.Sprint(v...)
	if sd.Client != nil {
		err := sd.Client.Info(msg)
		if err != nil {
			log.Fatal(err)
		}
		sd.Client.Flush()
	}
	log.Fatal(msg)
}
