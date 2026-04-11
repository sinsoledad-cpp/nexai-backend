package ioc

import (
	"nexai-backend/internal/user/repository/dao"

	"gorm.io/gorm"
)

func InitDatabase(db *gorm.DB) error {
	return db.AutoMigrate(
		&dao.User{},
		// 其他模块的表结构体可以在这里添加
	)
}
