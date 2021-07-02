package tests

import (
	"github.com/cactus/go-statsd-client/statsd"
	"go.uber.org/zap"
	"time"
)

type MockStatsdClient struct {
	Client *statsd.Statter
	Log    *zap.Logger
}

type MockStatsdStatter struct {

}


func (sd *MockStatsdClient) Init() {

}

func (sd *MockStatsdClient) Inc(statName string) {

}

func (sd *MockStatsdClient) IncRated(statName string, rate float32) {

}

func (sd *MockStatsdClient) Timing(statName string, value int64) {

}

func (sd *MockStatsdClient) TimingRated(statName string, value int64, rate float32) {

}

func (sd *MockStatsdClient) TimingDuration(statName string, value time.Duration) {

}

func (sd *MockStatsdClient) TimingDurationRated(statName string, value time.Duration, rate float32) {

}

func (sd *MockStatsdClient) Gauge(statName string, value int64) {

}

func (sd *MockStatsdClient) GaugeRated(statName string, value int64, rate float32) {

}
