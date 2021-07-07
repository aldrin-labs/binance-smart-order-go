package logging

import (
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"os"
)

func GetZapLogger() (*zap.Logger, error) {
	_ = godotenv.Load()
	if os.Getenv("LOCAL") == "true" {
		//logger, _ = zap.NewDevelopment()
		return zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Development:      true,
			Encoding:         "console",
			EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}.Build()
	}

	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.WarnLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()

}
