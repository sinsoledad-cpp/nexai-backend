package test

import (
	"log"
	"nexai-backend/cmd/server/ioc"
	"nexai-backend/pkg/logger"
	"testing"
)

func TestPostgreSQLConnection(t *testing.T) {
	// 初始化日志
	l := logger.NewNopLogger()
	
	// 尝试连接 PostgreSQL
	db := ioc.InitPostgreSQL(l)
	if db == nil {
		t.Fatal("Failed to connect to PostgreSQL")
	}
	
	// 测试数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get database instance: %v", err)
	}
	
	err = sqlDB.Ping()
	if err != nil {
		t.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	
	log.Println("PostgreSQL connection successful!")
}
