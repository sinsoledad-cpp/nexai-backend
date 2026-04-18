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
	FindByUserId(ctx context.Context, userId int64) ([]domain.Resume, error)
	UpdateStatus(ctx context.Context, id int64, status domain.ResumeStatus) error
	UpdateParsed(ctx context.Context, id int64, parsed domain.ParsedResume) error
	UpdateRawText(ctx context.Context, id int64, rawText string) error
	UpdateScore(ctx context.Context, id int64, score domain.ScoreResult) error
	UpdateOptimization(ctx context.Context, id int64, optimization domain.OptimizationResult) error
	Delete(ctx context.Context, id int64) error
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

func (c *CachedResumeRepository) FindByUserId(ctx context.Context, userId int64) ([]domain.Resume, error) {
	resumes, err := c.dao.FindByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Resume, 0, len(resumes))
	for _, r := range resumes {
		result = append(result, c.toDomain(r))
	}
	return result, nil
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

func (c *CachedResumeRepository) UpdateOptimization(ctx context.Context, id int64, optimization domain.OptimizationResult) error {
	data, err := json.Marshal(optimization)
	if err != nil {
		return err
	}
	return c.dao.UpdateOptimization(ctx, id, string(data))
}

func (c *CachedResumeRepository) Delete(ctx context.Context, id int64) error {
	return c.dao.Delete(ctx, id)
}

func (c *CachedResumeRepository) toEntity(r domain.Resume) dao.Resume {
	parsed, _ := json.Marshal(r.Parsed)
	var score string
	if r.Score.OverallScore > 0 {
		scoreBytes, _ := json.Marshal(r.Score)
		score = string(scoreBytes)
	}
	var optimization string
	if len(r.Optimization.Diagnoses) > 0 || r.Optimization.JdMatch.MatchScore > 0 {
		optBytes, _ := json.Marshal(r.Optimization)
		optimization = string(optBytes)
	}
	return dao.Resume{
		ID:           r.ID,
		UserID:       r.UserID,
		FileName:     r.FileName,
		FileURL:      r.FileURL,
		FileType:     r.FileType,
		RawText:      r.RawText,
		Parsed:       string(parsed),
		Status:       int(r.Status),
		Score:        score,
		Optimization: optimization,
	}
}

func (c *CachedResumeRepository) toDomain(r dao.Resume) domain.Resume {
	var parsed domain.ParsedResume
	if r.Parsed != "" {
		_ = json.Unmarshal([]byte(r.Parsed), &parsed)
	}
	var score domain.ScoreResult
	if r.Score != "" {
		_ = json.Unmarshal([]byte(r.Score), &score)
	}
	var optimization domain.OptimizationResult
	if r.Optimization != "" {
		_ = json.Unmarshal([]byte(r.Optimization), &optimization)
	}
	return domain.Resume{
		ID:           r.ID,
		UserID:       r.UserID,
		FileName:     r.FileName,
		FileURL:      r.FileURL,
		FileType:     r.FileType,
		RawText:      r.RawText,
		Parsed:       parsed,
		Score:        score,
		Optimization: optimization,
		Status:       domain.ResumeStatus(r.Status),
		Ctime:        time.UnixMilli(r.Ctime),
		Utime:        time.UnixMilli(r.Utime),
	}
}
