package service

import (
	"context"
	"encoding/json"
	"fmt"
	"nexai-backend/internal/resume/domain"
	"nexai-backend/internal/resume/repository"
	"nexai-backend/internal/resume/repository/dao"
	"nexai-backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
)

// 错误定义
var (
	ErrResumeNotFound      = repository.ErrResumeNotFound // 简历不存在
	ErrFileTypeUnsupported = fmt.Errorf("不支持的文件类型")       // 不支持的文件类型
	ErrFileTooLarge        = fmt.Errorf("文件大小超出限制")       // 文件大小超出限制
	ErrParseFailed         = fmt.Errorf("简历解析失败")         // 简历解析失败
	ErrNotParsed           = fmt.Errorf("简历尚未解析")         // 简历尚未解析
)

// 最大文件大小：10MB
const maxFileSize = 10 << 20

// ResumeService 简历服务接口
type ResumeService interface {
	Upload(ctx context.Context, uid int64, fileName string, fileData []byte) (domain.Resume, error)
	Parse(ctx context.Context, uid int64, fileID int64) (domain.ParsedResume, error)
	Correct(ctx context.Context, uid int64, fileID int64, parsed domain.ParsedResume) (domain.ParsedResume, error)
	Score(ctx context.Context, uid int64, fileID int64, targetPosition string) (domain.ScoreResult, error)
	Optimize(ctx context.Context, uid int64, fileID int64, targetPosition string, jd string) (domain.OptimizationResult, error)
	FindById(ctx context.Context, id int64) (domain.Resume, error)
	List(ctx context.Context, uid int64) ([]domain.Resume, error)
	Delete(ctx context.Context, uid int64, id int64) error
	ExportMarkdown(ctx context.Context, uid int64, id int64) (string, error)
}

// DefaultResumeService 简历服务默认实现
type DefaultResumeService struct {
	l          logger.Logger
	repo       repository.ResumeRepository
	versionDAO dao.ResumeVersionDAO
	parser     *ParseWorkflow
	scorer     *ScoringWorkflow
	optimizer  *OptimizationWorkflow
}

func NewResumeService(l logger.Logger, repo repository.ResumeRepository, versionDAO dao.ResumeVersionDAO, parser *ParseWorkflow, scorer *ScoringWorkflow, optimizer *OptimizationWorkflow) ResumeService {
	return &DefaultResumeService{
		l:          l,
		repo:       repo,
		versionDAO: versionDAO,
		parser:     parser,
		scorer:     scorer,
		optimizer:  optimizer,
	}
}

// Upload 处理简历上传：验证文件类型和大小，保存文件到磁盘，提取文本内容
func (svc *DefaultResumeService) Upload(ctx context.Context, uid int64, fileName string, fileData []byte) (domain.Resume, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != ".pdf" && ext != ".docx" {
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

	diskPath := "." + fileURL
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		svc.l.Error("创建存储目录失败", logger.Error(err))
	} else if err := os.WriteFile(diskPath, fileData, 0644); err != nil {
		svc.l.Error("保存简历文件失败", logger.Error(err), logger.String("path", diskPath))
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

// Parse 解析简历：提取结构化信息
func (svc *DefaultResumeService) Parse(ctx context.Context, uid int64, fileID int64) (domain.ParsedResume, error) {
	resume, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.ParsedResume{}, err
	}
	if resume.UserID != uid {
		return domain.ParsedResume{}, ErrResumeNotFound
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

// Correct 修正简历：更新解析后的数据
func (svc *DefaultResumeService) Correct(ctx context.Context, uid int64, fileID int64, parsed domain.ParsedResume) (domain.ParsedResume, error) {
	resume, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.ParsedResume{}, err
	}
	if resume.UserID != uid {
		return domain.ParsedResume{}, ErrResumeNotFound
	}

	if len(resume.Parsed.Education) > 0 || len(resume.Parsed.WorkExperience) > 0 || len(resume.Parsed.Projects) > 0 {
		oldData, _ := json.Marshal(resume.Parsed)
		_, _ = svc.versionDAO.Insert(ctx, dao.ResumeVersion{
			ResumeID: fileID,
			Parsed:   string(oldData),
		})
	}

	err = svc.repo.UpdateParsed(ctx, fileID, parsed)
	if err != nil {
		return domain.ParsedResume{}, err
	}

	return parsed, nil
}

// Score 评分简历：基于目标岗位进行多维度评分
func (svc *DefaultResumeService) Score(ctx context.Context, uid int64, fileID int64, targetPosition string) (domain.ScoreResult, error) {
	resume, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.ScoreResult{}, err
	}
	if resume.UserID != uid {
		return domain.ScoreResult{}, ErrResumeNotFound
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

// FindById 根据ID查找简历
func (svc *DefaultResumeService) FindById(ctx context.Context, id int64) (domain.Resume, error) {
	return svc.repo.FindById(ctx, id)
}

// Optimize 优化建议：基于目标岗位和JD提供优化分析
func (svc *DefaultResumeService) Optimize(ctx context.Context, uid int64, fileID int64, targetPosition string, jd string) (domain.OptimizationResult, error) {
	resume, err := svc.repo.FindById(ctx, fileID)
	if err != nil {
		return domain.OptimizationResult{}, err
	}
	if resume.UserID != uid {
		return domain.OptimizationResult{}, ErrResumeNotFound
	}

	if resume.Status < domain.StatusParsed {
		return domain.OptimizationResult{}, ErrNotParsed
	}

	parsedData, _ := json.Marshal(resume.Parsed)
	result, err := svc.optimizer.Evaluate(ctx, string(parsedData), targetPosition, jd)
	if err != nil {
		svc.l.Error("优化建议失败", logger.Error(err), logger.Int64("resumeId", fileID))
		return domain.OptimizationResult{}, err
	}

	result.ResumeID = fileID
	result.TargetPosition = targetPosition

	err = svc.repo.UpdateOptimization(ctx, fileID, result)
	if err != nil {
		svc.l.Error("保存优化结果失败", logger.Error(err))
	}

	return result, nil
}

func (svc *DefaultResumeService) List(ctx context.Context, uid int64) ([]domain.Resume, error) {
	return svc.repo.FindByUserId(ctx, uid)
}

func (svc *DefaultResumeService) Delete(ctx context.Context, uid int64, id int64) error {
	resume, err := svc.repo.FindById(ctx, id)
	if err != nil {
		return err
	}
	if resume.UserID != uid {
		return ErrResumeNotFound
	}
	return svc.repo.Delete(ctx, id)
}

func (svc *DefaultResumeService) ExportMarkdown(ctx context.Context, uid int64, id int64) (string, error) {
	resume, err := svc.repo.FindById(ctx, id)
	if err != nil {
		return "", err
	}
	if resume.UserID != uid {
		return "", ErrResumeNotFound
	}
	if resume.Status < domain.StatusParsed {
		return "", ErrNotParsed
	}
	return svc.toMarkdown(resume.Parsed), nil
}

func (svc *DefaultResumeService) toMarkdown(parsed domain.ParsedResume) string {
	var buf strings.Builder

	buf.WriteString("# ")
	buf.WriteString(parsed.PersonalInfo.Name)
	buf.WriteString("\n\n")

	if parsed.PersonalInfo.Phone != "" || parsed.PersonalInfo.Email != "" || parsed.PersonalInfo.Address != "" {
		var contacts []string
		if parsed.PersonalInfo.Phone != "" {
			contacts = append(contacts, parsed.PersonalInfo.Phone)
		}
		if parsed.PersonalInfo.Email != "" {
			contacts = append(contacts, parsed.PersonalInfo.Email)
		}
		if parsed.PersonalInfo.Address != "" {
			contacts = append(contacts, parsed.PersonalInfo.Address)
		}
		buf.WriteString(strings.Join(contacts, " | "))
		buf.WriteString("\n\n")
	}

	if parsed.PersonalInfo.Summary != "" {
		buf.WriteString("## 个人优势\n\n")
		buf.WriteString(parsed.PersonalInfo.Summary)
		buf.WriteString("\n\n")
	}

	if len(parsed.Education) > 0 {
		buf.WriteString("## 教育背景\n\n")
		for _, edu := range parsed.Education {
			buf.WriteString("- **")
			buf.WriteString(edu.School)
			buf.WriteString("** ")
			buf.WriteString(edu.Degree)
			buf.WriteString(" ")
			buf.WriteString(edu.Major)
			if edu.StartDate != "" || edu.EndDate != "" {
				buf.WriteString("  ")
				buf.WriteString(edu.StartDate)
				buf.WriteString(" - ")
				buf.WriteString(edu.EndDate)
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	if len(parsed.WorkExperience) > 0 {
		buf.WriteString("## 工作经历\n\n")
		for _, work := range parsed.WorkExperience {
			buf.WriteString("### ")
			buf.WriteString(work.Company)
			buf.WriteString(" - ")
			buf.WriteString(work.Position)
			if work.StartDate != "" || work.EndDate != "" {
				buf.WriteString("  ")
				buf.WriteString(work.StartDate)
				buf.WriteString(" - ")
				buf.WriteString(work.EndDate)
			}
			buf.WriteString("\n\n")
			if work.Description != "" {
				buf.WriteString(work.Description)
				buf.WriteString("\n\n")
			}
		}
	}

	if len(parsed.Projects) > 0 {
		buf.WriteString("## 项目经验\n\n")
		for _, proj := range parsed.Projects {
			buf.WriteString("### ")
			buf.WriteString(proj.Name)
			if proj.Role != "" {
				buf.WriteString(" - ")
				buf.WriteString(proj.Role)
			}
			if proj.StartDate != "" || proj.EndDate != "" {
				buf.WriteString("  ")
				buf.WriteString(proj.StartDate)
				buf.WriteString(" - ")
				buf.WriteString(proj.EndDate)
			}
			buf.WriteString("\n\n")
			if proj.Description != "" {
				buf.WriteString(proj.Description)
				buf.WriteString("\n\n")
			}
		}
	}

	if len(parsed.Skills) > 0 {
		buf.WriteString("## 专业技能\n\n")
		for _, skill := range parsed.Skills {
			buf.WriteString("- ")
			buf.WriteString(skill.Description)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}
