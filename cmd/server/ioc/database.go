package ioc

import (
	resumedao "nexai-backend/internal/resume/repository/dao"
	"nexai-backend/internal/user/repository/dao"

	"gorm.io/gorm"
)

func InitDatabase(db *gorm.DB) error {
	return db.AutoMigrate(
		&dao.User{},
		&resumedao.Resume{},
	)
}
