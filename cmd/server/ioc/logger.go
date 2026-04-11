package ioc

import (
	"nexai-backend/pkg/logger"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func InitLogger() logger.Logger {

	mode := viper.GetString("server.mode")
	if mode == "" {
		mode = "debug"
	}

	if err := InitZap(mode); err != nil {
		panic(err)
	}

	baseLogger := zap.L()
	return logger.NewZapLogger(baseLogger)
}
