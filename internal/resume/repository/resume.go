package repository

import (
	"context"
	"encoding/json"
	"errors"
	"nexai-backend/internal/resume/domain"
	"nexai-backend/internal/resume/repository/dao"
	"nexai-backend/pkg/logger"
	"time"
)

var (
	ErrResumeNotFound = errors.New("简历不存在")
)

type ResumeRepository interface {
	Create(ctx context.Context, resume domain.Resume) (domain.Resume, error)
	FindById(ctx context.Context, id int64) (domain.Resume, error)
	UpdateStatus(ctx context.Context, id int64, status domain.ResumeStatus) error
	UpdateParsed(ctx context.Context, id int64, parsed domain.ParsedResume) error
	UpdateRawText(ctx context.Context, id int64, rawText string) error
	UpdateScore(ctx context.Context, id int64, score domain.ScoreResult) error
}

type CachedResumeRepository struct {
	dao dao.ResumeDAO
	l   logger.Logger
}

func NewCachedResumeRepository(dao dao.ResumeDAO, l logger.Logger) ResumeRepository {
	return &CachedResumeRepository{
		dao: dao,
		l:   l,
	}
}

func (c *CachedResumeRepository) Create(ctx context.Context, resume domain.Resume) (domain.Resume, error) {
	r, err := c.dao.Insert(ctx, c.toEntity(resume))
	if err != nil {
		return domain.Resume{}, err
	}
	return c.toDomain(r), nil
}

func (c *CachedResumeRepository) FindById(ctx context.Context, id int64) (domain.Resume, error) {
	r, err := c.dao.FindById(ctx, id)
	if err != nil {
		if errors.Is(err, dao.ErrRecordNotFound) {
			return domain.Resume{}, ErrResumeNotFound
		}
		return domain.Resume{}, err
	}
	return c.toDomain(r), nil
}

func (c *CachedResumeRepository) UpdateStatus(ctx context.Context, id int64, status domain.ResumeStatus) error {
	return c.dao.UpdateStatus(ctx, id, int(status))
}

func (c *CachedResumeRepository) UpdateParsed(ctx context.Context, id int64, parsed domain.ParsedResume) error {
	data, err := json.Marshal(parsed)
	if err != nil {
		return err
	}
	return c.dao.UpdateParsed(ctx, id, string(data), int(domain.StatusParsed))
}

func (c *CachedResumeRepository) UpdateRawText(ctx context.Context, id int64, rawText string) error {
	return c.dao.UpdateRawText(ctx, id, rawText)
}

func (c *CachedResumeRepository) UpdateScore(ctx context.Context, id int64, score domain.ScoreResult) error {
	data, err := json.Marshal(score)
	if err != nil {
		return err
	}
	return c.dao.UpdateScore(ctx, id, string(data), int(domain.StatusScored))
}

func (c *CachedResumeRepository) toEntity(r domain.Resume) dao.Resume {
	parsed, _ := json.Marshal(r.Parsed)
	score, _ := json.Marshal(domain.ScoreResult{})
	if r.Status == domain.StatusScored {
		score, _ = json.Marshal(r.Parsed)
	}
	return dao.Resume{
		ID:       r.ID,
		UserID:   r.UserID,
		FileName: r.FileName,
		FileURL:  r.FileURL,
		FileType: r.FileType,
		RawText:  r.RawText,
		Parsed:   string(parsed),
		Status:   int(r.Status),
		Score:    string(score),
	}
}

func (c *CachedResumeRepository) toDomain(r dao.Resume) domain.Resume {
	var parsed domain.ParsedResume
	if r.Parsed != "" {
		_ = json.Unmarshal([]byte(r.Parsed), &parsed)
	}
	return domain.Resume{
		ID:       r.ID,
		UserID:   r.UserID,
		FileName: r.FileName,
		FileURL:  r.FileURL,
		FileType: r.FileType,
		RawText:  r.RawText,
		Parsed:   parsed,
		Status:   domain.ResumeStatus(r.Status),
		Ctime:    time.UnixMilli(r.Ctime),
		Utime:    time.UnixMilli(r.Utime),
	}
}
