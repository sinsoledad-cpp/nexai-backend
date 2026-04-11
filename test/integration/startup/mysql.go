package startup

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitMySQL() *gorm.DB {
	// 这里假设本地有一个可用的 MySQL 实例
	// 注意：实际项目中配置应从环境变量或配置文件读取

	// 1. 先连接默认的 mysql 数据库，用于创建 bedrock_integration_test
	dsn := "root:root@tcp(localhost:3306)/mysql?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 2. 创建测试数据库，防止污染现有库
	err = db.Exec("CREATE DATABASE IF NOT EXISTS bedrock_integration_test").Error
	if err != nil {
		panic(err)
	}

	// 3. 连接到 bedrock_integration_test
	dsn = "root:root@tcp(localhost:3306)/bedrock_integration_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}
