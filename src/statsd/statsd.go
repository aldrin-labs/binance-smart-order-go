package statsd_client

import (
	"fmt"
	"github.com/cactus/go-statsd-client/statsd"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

type StatsdClient struct {
	Client *statsd.Statter
	Log    *zap.Logger
}

var once sync.Once

func (sd *StatsdClient) Init() {
	if os.Getenv("LOCAL") == "true"{
		sd.Log, _ = zap.NewDevelopment()
	} else {
		sd.Log, _ = zap.NewProduction()
	}
	sd.Log = sd.Log.With(zap.String("logger", "statsd"))
	host := os.Getenv("STATSD_HOST")
	if host == "" {
		host = "graphite.infra"
	}
	// port := os.Getenv("STATSD_PORT")
	port := "8125"
	sd.Log.Info("connecting",
		zap.String("host", host),
		zap.String("port", port),
	)
	config := &statsd.ClientConfig{
		Address:       fmt.Sprintf("%s:%s", host, port),
		Prefix:        "strategy_service",
		FlushInterval: 1000 * time.Millisecond, // fixed max delay for alerts
	}
	client, err := statsd.NewClientWithConfig(config)
	if err != nil {
		sd.Log.Error("StatsD init error, disabling stats", zap.Error(err))
		return
	}
	sd.Client = &client
	sd.Log.Info("StatsD init successful.")
}

func (sd *StatsdClient) Inc(statName string) {
	if sd.Client != nil {
		err := (*sd.Client).Inc(statName, 1, 1.0)
		if err != nil {
			once.Do(func() {
				sd.Log.Error("Error on StatsD, further error messages supressed", zap.Error(err))
			})
		}
	}
}

func (sd *StatsdClient) IncRated(statName string, rate float32) {
	if sd.Client != nil {
		err := (*sd.Client).Inc(statName, 1, rate)
		if err != nil {
			once.Do(func() {
				sd.Log.Error("Error on StatsD, further error messages supressed", zap.Error(err))
			})
		}
	}
}

func (sd *StatsdClient) Timing(statName string, value int64) {
	if sd.Client != nil {
		err := (*sd.Client).Timing(statName, value, 1.0)
		if err != nil {
			once.Do(func() {
				sd.Log.Error("Error on StatsD, further error messages supressed", zap.Error(err))
			})
		}
	}
}

func (sd *StatsdClient) TimingRated(statName string, value int64, rate float32) {
	if sd.Client != nil {
		err := (*sd.Client).Timing(statName, value, rate)
		if err != nil {
			sd.Log.Error("Error on Statsd Timing", zap.Error(err))
		}
	}
}

func (sd *StatsdClient) TimingDuration(statName string, value time.Duration) {
	if sd.Client != nil {
		err := (*sd.Client).TimingDuration(statName, value, 1.0)
		if err != nil {
			once.Do(func() {
				sd.Log.Error("Error on StatsD, further error messages supressed", zap.Error(err))
			})
		}
	}
}

func (sd *StatsdClient) TimingDurationRated(statName string, value time.Duration, rate float32) {
	if sd.Client != nil {
		err := (*sd.Client).TimingDuration(statName, value, rate)
		if err != nil {
			sd.Log.Error("Error on Statsd TimeDuration", zap.Error(err))
		}
	}
}

func (sd *StatsdClient) Gauge(statName string, value int64) {
	if sd.Client != nil {
		err := (*sd.Client).Gauge(statName, value, 1.0)
		if err != nil {
			once.Do(func() {
				sd.Log.Error("Error on StatsD, further error messages supressed", zap.Error(err))
			})
		}
	}
}

func (sd *StatsdClient) GaugeRated(statName string, value int64, rate float32) {
	if sd.Client != nil {
		err := (*sd.Client).Gauge(statName, value, rate)
		if err != nil {
			once.Do(func() {
				sd.Log.Error("Error on StatsD, further error messages supressed", zap.Error(err))
			})
		}
	}
}
