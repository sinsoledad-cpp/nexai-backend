package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"nexai-backend/internal/common/jwt"
	"nexai-backend/internal/common/router"
	"nexai-backend/internal/resume/domain"
	"nexai-backend/internal/resume/handler/dto"
	"nexai-backend/internal/resume/handler/errs"
	resumeservice "nexai-backend/internal/resume/service"
	"nexai-backend/pkg/ginx"
	"nexai-backend/pkg/logger"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var _ router.Handler = (*ResumeHandler)(nil)

// ResumeHandler 简历处理器
type ResumeHandler struct {
	log logger.Logger               // 日志记录器
	svc resumeservice.ResumeService // 简历服务
}

// NewResumeHandler 创建简历处理器实例
func NewResumeHandler(log logger.Logger, svc resumeservice.ResumeService) *ResumeHandler {
	return &ResumeHandler{
		log: log,
		svc: svc,
	}
}

// RegisterRoutes 注册路由
func (h *ResumeHandler) RegisterRoutes(e *gin.Engine) {
	v1 := e.Group("/v1")
	resumes := v1.Group("/resumes")

	resumes.POST("/upload", ginx.WrapClaims(h.Upload))   // 上传简历
	resumes.POST("/parse", ginx.WrapBody(h.Parse))       // 解析简历
	resumes.POST("/correct", ginx.WrapBody(h.Correct))   // 修正简历
	resumes.POST("/score", ginx.WrapBody(h.Score))       // 评分简历
	resumes.POST("/optimize", ginx.WrapBody(h.Optimize)) // 优化建议
	resumes.GET("/:id", ginx.Wrap(h.GetById))            // 获取简历
}

// Upload 处理文件上传
func (h *ResumeHandler) Upload(ctx *gin.Context, uc jwt.UserClaims) (ginx.Result, error) {
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		return ginx.Result{
			Code: errs.ResumeInvalidInput,
			Msg:  "无法获取上传文件",
		}, err
	}
	defer file.Close()

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" && ext != ".docx" && ext != ".doc" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return ginx.Result{
			Code: errs.ResumeFileTypeUnsupported,
			Msg:  "不支持的文件类型，请上传 PDF、DOCX 或图片文件",
		}, nil
	}

	// 读取文件数据
	fileData, err := io.ReadAll(file)
	if err != nil {
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "读取文件失败",
		}, err
	}

	// 检查文件大小
	if len(fileData) > 10<<20 {
		return ginx.Result{
			Code: errs.ResumeFileTooLarge,
			Msg:  "文件大小不能超过10MB",
		}, nil
	}

	// 调用服务上传文件
	resume, err := h.svc.Upload(ctx.Request.Context(), uc.Uid, header.Filename, fileData)
	if err != nil {
		if errors.Is(err, resumeservice.ErrFileTypeUnsupported) {
			return ginx.Result{
				Code: errs.ResumeFileTypeUnsupported,
				Msg:  "不支持的文件类型",
			}, err
		}
		if errors.Is(err, resumeservice.ErrFileTooLarge) {
			return ginx.Result{
				Code: errs.ResumeFileTooLarge,
				Msg:  "文件大小超出限制",
			}, err
		}
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "上传失败",
		}, err
	}

	return ginx.Result{
		Code: http.StatusCreated,
		Msg:  "上传成功",
		Data: dto.UploadResponse{
			FileID:   resume.ID,
			FileName: resume.FileName,
			FileType: resume.FileType,
		},
	}, nil
}

// Parse 处理简历解析
func (h *ResumeHandler) Parse(ctx *gin.Context, req dto.ParseRequest) (ginx.Result, error) {
	parsed, err := h.svc.Parse(ctx.Request.Context(), 0, req.FileID)
	if err != nil {
		if errors.Is(err, resumeservice.ErrResumeNotFound) {
			return ginx.Result{
				Code: errs.ResumeFileNotFound,
				Msg:  "简历文件不存在",
			}, err
		}
		if errors.Is(err, resumeservice.ErrParseFailed) {
			return ginx.Result{
				Code: errs.ResumeParseFailed,
				Msg:  "简历解析失败",
			}, err
		}
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "系统错误",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "解析成功",
		Data: h.toParseResponse(parsed),
	}, nil
}

// Correct 处理简历修正
func (h *ResumeHandler) Correct(ctx *gin.Context, req dto.CorrectRequest) (ginx.Result, error) {
	parsed := h.toDomainParsed(req.Parsed)
	result, err := h.svc.Correct(ctx.Request.Context(), 0, req.FileID, parsed)
	if err != nil {
		if errors.Is(err, resumeservice.ErrResumeNotFound) {
			return ginx.Result{
				Code: errs.ResumeFileNotFound,
				Msg:  "简历文件不存在",
			}, err
		}
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "系统错误",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "修正成功",
		Data: h.toParseResponse(result),
	}, nil
}

// Score 处理简历评分
func (h *ResumeHandler) Score(ctx *gin.Context, req dto.ScoreRequest) (ginx.Result, error) {
	result, err := h.svc.Score(ctx.Request.Context(), 0, req.FileID, req.TargetPosition)
	if err != nil {
		if errors.Is(err, resumeservice.ErrResumeNotFound) {
			return ginx.Result{
				Code: errs.ResumeFileNotFound,
				Msg:  "简历文件不存在",
			}, err
		}
		if errors.Is(err, resumeservice.ErrNotParsed) {
			return ginx.Result{
				Code: errs.ResumeNotParsed,
				Msg:  "简历尚未解析，请先解析简历",
			}, err
		}
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "评分失败",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "评分完成",
		Data: h.toScoreResponse(result),
	}, nil
}

// GetById 根据ID获取简历
func (h *ResumeHandler) GetById(ctx *gin.Context) (ginx.Result, error) {
	idStr := ctx.Param("id")
	var id int64
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return ginx.Result{
			Code: errs.ResumeInvalidInput,
			Msg:  "无效的简历ID",
		}, err
	}

	resume, err := h.svc.FindById(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, resumeservice.ErrResumeNotFound) {
			return ginx.Result{
				Code: errs.ResumeFileNotFound,
				Msg:  "简历不存在",
			}, err
		}
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "系统错误",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "获取成功",
		Data: resume,
	}, nil
}

// Optimize 处理优化建议
func (h *ResumeHandler) Optimize(ctx *gin.Context, req dto.OptimizeRequest) (ginx.Result, error) {
	result, err := h.svc.Optimize(ctx.Request.Context(), 0, req.FileID, req.TargetPosition, req.JD)
	if err != nil {
		if errors.Is(err, resumeservice.ErrResumeNotFound) {
			return ginx.Result{
				Code: errs.ResumeFileNotFound,
				Msg:  "简历文件不存在",
			}, err
		}
		if errors.Is(err, resumeservice.ErrNotParsed) {
			return ginx.Result{
				Code: errs.ResumeNotParsed,
				Msg:  "简历尚未解析，请先解析简历",
			}, err
		}
		return ginx.Result{
			Code: errs.ResumeOptimizeFailed,
			Msg:  "优化建议生成失败",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "优化建议生成完成",
		Data: h.toOptimizeResponse(result),
	}, nil
}

// toParseResponse 将领域模型转换为响应DTO
func (h *ResumeHandler) toParseResponse(parsed domain.ParsedResume) dto.ParseResponse {
	resp := dto.ParseResponse{
		PersonalInfo: dto.PersonalInfoDTO{
			Name:    parsed.PersonalInfo.Name,
			Phone:   parsed.PersonalInfo.Phone,
			Email:   parsed.PersonalInfo.Email,
			Address: parsed.PersonalInfo.Address,
			Summary: parsed.PersonalInfo.Summary,
		},
		Skills: parsed.Skills,
	}

	for _, edu := range parsed.Education {
		resp.Education = append(resp.Education, dto.EducationDTO{
			School:    edu.School,
			Degree:    edu.Degree,
			Major:     edu.Major,
			StartDate: edu.StartDate,
			EndDate:   edu.EndDate,
		})
	}

	for _, work := range parsed.WorkExperience {
		resp.WorkExperience = append(resp.WorkExperience, dto.WorkExperienceDTO{
			Company:     work.Company,
			Position:    work.Position,
			StartDate:   work.StartDate,
			EndDate:     work.EndDate,
			Description: work.Description,
		})
	}

	for _, proj := range parsed.Projects {
		resp.Projects = append(resp.Projects, dto.ProjectDTO{
			Name:        proj.Name,
			Role:        proj.Role,
			StartDate:   proj.StartDate,
			EndDate:     proj.EndDate,
			Description: proj.Description,
		})
	}

	return resp
}

// toDomainParsed 将响应DTO转换为领域模型
func (h *ResumeHandler) toDomainParsed(dtoParsed dto.ParseResponse) domain.ParsedResume {
	parsed := domain.ParsedResume{
		PersonalInfo: domain.PersonalInfo{
			Name:    dtoParsed.PersonalInfo.Name,
			Phone:   dtoParsed.PersonalInfo.Phone,
			Email:   dtoParsed.PersonalInfo.Email,
			Address: dtoParsed.PersonalInfo.Address,
			Summary: dtoParsed.PersonalInfo.Summary,
		},
		Skills: dtoParsed.Skills,
	}

	for _, edu := range dtoParsed.Education {
		parsed.Education = append(parsed.Education, domain.Education{
			School:    edu.School,
			Degree:    edu.Degree,
			Major:     edu.Major,
			StartDate: edu.StartDate,
			EndDate:   edu.EndDate,
		})
	}

	for _, work := range dtoParsed.WorkExperience {
		parsed.WorkExperience = append(parsed.WorkExperience, domain.WorkExperience{
			Company:     work.Company,
			Position:    work.Position,
			StartDate:   work.StartDate,
			EndDate:     work.EndDate,
			Description: work.Description,
		})
	}

	for _, proj := range dtoParsed.Projects {
		parsed.Projects = append(parsed.Projects, domain.Project{
			Name:        proj.Name,
			Role:        proj.Role,
			StartDate:   proj.StartDate,
			EndDate:     proj.EndDate,
			Description: proj.Description,
		})
	}

	return parsed
}

// toScoreResponse 将评分结果转换为响应DTO
func (h *ResumeHandler) toScoreResponse(result domain.ScoreResult) dto.ScoreResponse {
	return dto.ScoreResponse{
		ResumeID:        result.ResumeID,
		OverallScore:    result.OverallScore,
		TargetPosition:  result.TargetPosition,
		Recommendations: result.Recommendations,
		Dimensions: dto.ScoreDimensionsDTO{
			Completeness: dto.DimensionScoreDTO{
				Score:   result.Dimensions.Completeness.Score,
				Reasons: result.Dimensions.Completeness.Reasons,
			},
			Professionalism: dto.DimensionScoreDTO{
				Score:   result.Dimensions.Professionalism.Score,
				Reasons: result.Dimensions.Professionalism.Reasons,
			},
			Quantification: dto.DimensionScoreDTO{
				Score:   result.Dimensions.Quantification.Score,
				Reasons: result.Dimensions.Quantification.Reasons,
			},
			Format: dto.DimensionScoreDTO{
				Score:   result.Dimensions.Format.Score,
				Reasons: result.Dimensions.Format.Reasons,
			},
		},
	}
}

// toOptimizeResponse 将优化结果转换为响应DTO
func (h *ResumeHandler) toOptimizeResponse(result domain.OptimizationResult) dto.OptimizeResponse {
	diagnoses := make([]dto.DiagnosisDTO, 0, len(result.Diagnoses))
	for _, d := range result.Diagnoses {
		diagnoses = append(diagnoses, dto.DiagnosisDTO{
			Target:     d.Target,
			Issue:      d.Issue,
			Severity:   d.Severity,
			Suggestion: d.Suggestion,
			Type:       d.Type,
		})
	}

	starRewrites := make([]dto.StarRewriteDTO, 0, len(result.StarRewrites))
	for _, r := range result.StarRewrites {
		starRewrites = append(starRewrites, dto.StarRewriteDTO{
			Original:  r.Original,
			Rewritten: r.Rewritten,
			Section:   r.Section,
		})
	}

	gapAnalysis := make([]dto.GapSuggestionDTO, 0, len(result.JdMatch.GapAnalysis))
	for _, g := range result.JdMatch.GapAnalysis {
		gapAnalysis = append(gapAnalysis, dto.GapSuggestionDTO{
			Skill:      g.Skill,
			Importance: g.Importance,
			Suggestion: g.Suggestion,
			Type:       g.Type,
		})
	}

	return dto.OptimizeResponse{
		ResumeID:       result.ResumeID,
		TargetPosition: result.TargetPosition,
		Diagnoses:      diagnoses,
		StarRewrites:   starRewrites,
		JdMatch: dto.JdMatchResultDTO{
			MatchScore:    result.JdMatch.MatchScore,
			MatchedSkills: result.JdMatch.MatchedSkills,
			MissingSkills: result.JdMatch.MissingSkills,
			GapAnalysis:   gapAnalysis,
		},
	}
}
