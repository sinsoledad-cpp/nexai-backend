package ioc

import (
	"nexai-backend/pkg/logger"

	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	glogger "gorm.io/gorm/logger"
)

func InitPostgreSQL(l logger.Logger) *gorm.DB {
	type postgresConfig struct {
		DSN string `yaml:"dsn"`
	}
	var cfg postgresConfig = postgresConfig{
		DSN: "host=localhost user=postgres password=postgres dbname=nexai port=5432 sslmode=disable TimeZone=Asia/Shanghai",
	}
	if err := viper.UnmarshalKey("postgres", &cfg); err != nil {
		panic(err)
	}
	db, err := gorm.Open(postgres.Open(cfg.DSN),
		&gorm.Config{
			Logger: glogger.New(gormLoggerFunc(func(msg string, fields ...logger.Field) {
				l.Debug(msg, fields...)
			}), glogger.Config{
				// 慢查询
				SlowThreshold: 0,
				LogLevel:      glogger.Info,
			}),
		},
	)
	if err != nil {
		panic(err)
	}
	if err = InitDatabase(db); err != nil {
		panic(err)
	}
	return db
}
