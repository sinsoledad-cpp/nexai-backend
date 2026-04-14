package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Resume struct {
	ID       int64  `gorm:"primaryKey,autoIncrement"`
	UserID   int64  `gorm:"index"`
	FileName string `gorm:"type=varchar(512)"`
	FileURL  string `gorm:"type=varchar(1024)"`
	FileType string `gorm:"type=varchar(32)"`
	RawText  string `gorm:"type:text"`
	Parsed   string `gorm:"type:jsonb"`
	Status   int    `gorm:"type:smallint;default:0"`
	Score    string `gorm:"type:jsonb"`
	Ctime    int64
	Utime    int64
}

var (
	ErrRecordNotFound = gorm.ErrRecordNotFound
)

type ResumeDAO interface {
	Insert(ctx context.Context, resume Resume) (Resume, error)
	FindById(ctx context.Context, id int64) (Resume, error)
	UpdateStatus(ctx context.Context, id int64, status int) error
	UpdateParsed(ctx context.Context, id int64, parsed string, status int) error
	UpdateRawText(ctx context.Context, id int64, rawText string) error
	UpdateScore(ctx context.Context, id int64, score string, status int) error
}

type GORMResumeDAO struct {
	db *gorm.DB
}

func NewGORMResumeDAO(db *gorm.DB) ResumeDAO {
	return &GORMResumeDAO{db: db}
}

func (g *GORMResumeDAO) Insert(ctx context.Context, resume Resume) (Resume, error) {
	now := time.Now().UnixMilli()
	resume.Ctime = now
	resume.Utime = now
	err := g.db.WithContext(ctx).Create(&resume).Error
	return resume, err
}

func (g *GORMResumeDAO) FindById(ctx context.Context, id int64) (Resume, error) {
	var r Resume
	err := g.db.WithContext(ctx).Where("id = ?", id).First(&r).Error
	return r, err
}

func (g *GORMResumeDAO) UpdateStatus(ctx context.Context, id int64, status int) error {
	return g.db.WithContext(ctx).Model(&Resume{}).Where("id = ?", id).
		Updates(map[string]any{
			"status": status,
			"utime":  time.Now().UnixMilli(),
		}).Error
}

func (g *GORMResumeDAO) UpdateParsed(ctx context.Context, id int64, parsed string, status int) error {
	return g.db.WithContext(ctx).Model(&Resume{}).Where("id = ?", id).
		Updates(map[string]any{
			"parsed": parsed,
			"status": status,
			"utime":  time.Now().UnixMilli(),
		}).Error
}

func (g *GORMResumeDAO) UpdateRawText(ctx context.Context, id int64, rawText string) error {
	return g.db.WithContext(ctx).Model(&Resume{}).Where("id = ?", id).
		Updates(map[string]any{
			"raw_text": rawText,
			"utime":    time.Now().UnixMilli(),
		}).Error
}

func (g *GORMResumeDAO) UpdateScore(ctx context.Context, id int64, score string, status int) error {
	return g.db.WithContext(ctx).Model(&Resume{}).Where("id = ?", id).
		Updates(map[string]any{
			"score":  score,
			"status": status,
			"utime":  time.Now().UnixMilli(),
		}).Error
}

func (r Resume) ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
