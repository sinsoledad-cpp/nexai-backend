package startup

import "nexai-backend/pkg/logger"

func InitLogger() logger.Logger {
	return logger.NewNopLogger()
}
