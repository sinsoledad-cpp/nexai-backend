package service

import (
	"context"
	"encoding/json"
	"fmt"
	"nexai-backend/internal/resume/domain"
	"nexai-backend/internal/resume/repository"
	"nexai-backend/pkg/logger"
	"path/filepath"
	"strings"
)

var (
	ErrResumeNotFound      = repository.ErrResumeNotFound
	ErrFileTypeUnsupported = fmt.Errorf("不支持的文件类型")
	ErrFileTooLarge        = fmt.Errorf("文件大小超出限制")
	ErrParseFailed         = fmt.Errorf("简历解析失败")
	ErrNotParsed           = fmt.Errorf("简历尚未解析")
)

const maxFileSize = 10 << 20

type ResumeService interface {
	Upload(ctx context.Context, uid int64, fileName string, fileData []byte) (domain.Resume, error)
	Parse(ctx context.Context, uid int64, fileID int64) (domain.ParsedResume, error)
	Correct(ctx context.Context, uid int64, fileID int64, parsed domain.ParsedResume) (domain.ParsedResume, error)
	Score(ctx context.Context, uid int64, fileID int64, targetPosition string) (domain.ScoreResult, error)
	FindById(ctx context.Context, id int64) (domain.Resume, error)
}

type DefaultResumeService struct {
	l      logger.Logger
	repo   repository.ResumeRepository
	parser *ParseWorkflow
	scorer *ScoringWorkflow
}

func NewResumeService(l logger.Logger, repo repository.ResumeRepository, parser *ParseWorkflow, scorer *ScoringWorkflow) ResumeService {
	return &DefaultResumeService{
		l:      l,
		repo:   repo,
		parser: parser,
		scorer: scorer,
	}
}

func (svc *DefaultResumeService) Upload(ctx context.Context, uid int64, fileName string, fileData []byte) (domain.Resume, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != ".pdf" && ext != ".docx" && ext != ".doc" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return domain.Resume{}, ErrFileTypeUnsupported
	}

	if len(fileData) > maxFileSize {
		return domain.Resume{}, ErrFileTooLarge
	}

	fileURL := fmt.Sprintf("/storage/resumes/%d_%s", uid, fileName)

	resume := domain.Resume{
		UserID:   uid,
		FileName: fileName,
		FileURL:  fileURL,
		FileType: ext,
		Status:   domain.StatusUploaded,
	}

	r, err := svc.repo.Create(ctx, resume)
	if err != nil {
		return domain.Resume{}, err
	}

	rawText, err := svc.parser.ExtractText(ctx, ext, fileData)
	if err != nil {
		svc.l.Error("提取文本失败", logger.Error(err), logger.Int64("resumeId", r.ID))
		_ = svc.repo.UpdateStatus(ctx, r.ID, domain.StatusParseFailed)
		return domain.Resume{}, ErrParseFailed
	}

	err = svc.repo.UpdateRawText(ctx, r.ID, rawText)
	if err != nil {
		svc.l.Error("保存原始文本失败", logger.Error(err))
		return domain.Resume{}, err
	}

	r.RawText = rawText
	return r, nil
}

func (svc *DefaultResumeService) Parse(ctx context.Context, uid int64, fileID int64) (domain.ParsedResume, error) {
	resume, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.ParsedResume{}, err
	}

	if resume.RawText == "" {
		return domain.ParsedResume{}, ErrParseFailed
	}

	_ = svc.repo.UpdateStatus(ctx, fileID, domain.StatusParsing)

	parsed, err := svc.parser.Run(ctx, resume.RawText)
	if err != nil {
		svc.l.Error("解析简历失败", logger.Error(err), logger.Int64("resumeId", fileID))
		_ = svc.repo.UpdateStatus(ctx, fileID, domain.StatusParseFailed)
		return domain.ParsedResume{}, ErrParseFailed
	}

	err = svc.repo.UpdateParsed(ctx, fileID, parsed)
	if err != nil {
		svc.l.Error("保存解析结果失败", logger.Error(err))
		return domain.ParsedResume{}, err
	}

	return parsed, nil
}

func (svc *DefaultResumeService) Correct(ctx context.Context, uid int64, fileID int64, parsed domain.ParsedResume) (domain.ParsedResume, error) {
	_, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.ParsedResume{}, err
	}

	err = svc.repo.UpdateParsed(ctx, fileID, parsed)
	if err != nil {
		return domain.ParsedResume{}, err
	}

	return parsed, nil
}

func (svc *DefaultResumeService) Score(ctx context.Context, uid int64, fileID int64, targetPosition string) (domain.ScoreResult, error) {
	resume, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.ScoreResult{}, err
	}

	if resume.Status < domain.StatusParsed {
		return domain.ScoreResult{}, ErrNotParsed
	}

	parsedData, _ := json.Marshal(resume.Parsed)
	result, err := svc.scorer.Evaluate(ctx, string(parsedData), targetPosition)
	if err != nil {
		svc.l.Error("评分失败", logger.Error(err), logger.Int64("resumeId", fileID))
		return domain.ScoreResult{}, err
	}

	result.ResumeID = fileID
	result.TargetPosition = targetPosition

	err = svc.repo.UpdateScore(ctx, fileID, result)
	if err != nil {
		svc.l.Error("保存评分结果失败", logger.Error(err))
		return domain.ScoreResult{}, err
	}

	return result, nil
}

func (svc *DefaultResumeService) FindById(ctx context.Context, id int64) (domain.Resume, error) {
	return svc.repo.FindById(ctx, id)
}
