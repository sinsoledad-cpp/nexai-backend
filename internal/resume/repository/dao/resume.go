package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Resume struct {
	ID           int64  `gorm:"primaryKey,autoIncrement"`
	UserID       int64  `gorm:"index"`
	FileName     string `gorm:"type=varchar(512)"`
	FileURL      string `gorm:"type=varchar(1024)"`
	FileType     string `gorm:"type=varchar(32)"`
	RawText      string `gorm:"type:text"`
	Parsed       string `gorm:"type:jsonb"`
	Status       int    `gorm:"type:smallint;default:0"`
	Score        string `gorm:"type:jsonb"`
	Optimization string `gorm:"type:jsonb"`
	Ctime        int64
	Utime        int64
}

var (
	ErrRecordNotFound = gorm.ErrRecordNotFound
)

type ResumeDAO interface {
	Insert(ctx context.Context, resume Resume) (Resume, error)
	FindById(ctx context.Context, id int64) (Resume, error)
	FindByUserId(ctx context.Context, userId int64) ([]Resume, error)
	UpdateStatus(ctx context.Context, id int64, status int) error
	UpdateParsed(ctx context.Context, id int64, parsed string, status int) error
	UpdateRawText(ctx context.Context, id int64, rawText string) error
	UpdateScore(ctx context.Context, id int64, score string, status int) error
	UpdateOptimization(ctx context.Context, id int64, optimization string) error
	Delete(ctx context.Context, id int64) error
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

func (g *GORMResumeDAO) FindByUserId(ctx context.Context, userId int64) ([]Resume, error) {
	var resumes []Resume
	err := g.db.WithContext(ctx).Where("user_id = ?", userId).Order("ctime DESC").Find(&resumes).Error
	return resumes, err
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

func (g *GORMResumeDAO) UpdateOptimization(ctx context.Context, id int64, optimization string) error {
	return g.db.WithContext(ctx).Model(&Resume{}).Where("id = ?", id).
		Updates(map[string]any{
			"optimization": optimization,
			"utime":        time.Now().UnixMilli(),
		}).Error
}

func (g *GORMResumeDAO) Delete(ctx context.Context, id int64) error {
	return g.db.WithContext(ctx).Where("id = ?", id).Delete(&Resume{}).Error
}

func (r Resume) ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

type ResumeVersion struct {
	ID       int64  `gorm:"primaryKey,autoIncrement"`
	ResumeID int64  `gorm:"index"`
	Parsed   string `gorm:"type:jsonb"`
	Ctime    int64
}

type ResumeVersionDAO interface {
	Insert(ctx context.Context, version ResumeVersion) (ResumeVersion, error)
	FindByResumeId(ctx context.Context, resumeId int64) ([]ResumeVersion, error)
}

type GORMResumeVersionDAO struct {
	db *gorm.DB
}

func NewGORMResumeVersionDAO(db *gorm.DB) ResumeVersionDAO {
	return &GORMResumeVersionDAO{db: db}
}

func (g *GORMResumeVersionDAO) Insert(ctx context.Context, version ResumeVersion) (ResumeVersion, error) {
	version.Ctime = time.Now().UnixMilli()
	err := g.db.WithContext(ctx).Create(&version).Error
	return version, err
}

func (g *GORMResumeVersionDAO) FindByResumeId(ctx context.Context, resumeId int64) ([]ResumeVersion, error) {
	var versions []ResumeVersion
	err := g.db.WithContext(ctx).Where("resume_id = ?", resumeId).Order("ctime DESC").Find(&versions).Error
	return versions, err
}
