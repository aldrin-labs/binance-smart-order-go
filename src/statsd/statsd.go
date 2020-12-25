package statsd_client

import (
	"fmt"
	"github.com/cactus/go-statsd-client/statsd"
	"go.uber.org/zap"
	"os"
	"time"
)

type StatsdClient struct {
	Client *statsd.Statter
	Log    *zap.Logger
}

func (sd *StatsdClient) Init() {
	sd.Log, _ = zap.NewProduction()
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
			sd.Log.Error("Error on Statsd Inc", zap.Error(err))
		}
	}
}

func (sd *StatsdClient) Timing(statName string, value int64) {
	if sd.Client != nil {
		err := (*sd.Client).Timing(statName, value, 1.0)
		if err != nil {
			sd.Log.Error("Error on Statsd Timing", zap.Error(err))
		}
	}
}

func (sd *StatsdClient) TimingDuration(statName string, value time.Duration) {
	if sd.Client != nil {
		err := (*sd.Client).TimingDuration(statName, value, 1.0)
		if err != nil {
			sd.Log.Error("Error on Statsd TimeDuration", zap.Error(err))
		}
	}
}

func (sd *StatsdClient) Gauge(statName string, value int64) {
	if sd.Client != nil {
		err := (*sd.Client).Gauge(statName, value, 1.0)
		if err != nil {
			sd.Log.Error("Error on Statsd Gauge", zap.Error(err))
		}
	}
}
