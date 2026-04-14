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

type ResumeHandler struct {
	log logger.Logger
	svc resumeservice.ResumeService
}

func NewResumeHandler(log logger.Logger, svc resumeservice.ResumeService) *ResumeHandler {
	return &ResumeHandler{
		log: log,
		svc: svc,
	}
}

func (h *ResumeHandler) RegisterRoutes(e *gin.Engine) {
	v1 := e.Group("/v1")
	resumes := v1.Group("/resumes")

	resumes.POST("/upload", ginx.WrapClaims(h.Upload))
	resumes.POST("/parse", ginx.WrapBody(h.Parse))
	resumes.POST("/correct", ginx.WrapBody(h.Correct))
	resumes.POST("/score", ginx.WrapBody(h.Score))
	resumes.GET("/:id", ginx.Wrap(h.GetById))
}

func (h *ResumeHandler) Upload(ctx *gin.Context, uc jwt.UserClaims) (ginx.Result, error) {
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		return ginx.Result{
			Code: errs.ResumeInvalidInput,
			Msg:  "无法获取上传文件",
		}, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" && ext != ".docx" && ext != ".doc" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return ginx.Result{
			Code: errs.ResumeFileTypeUnsupported,
			Msg:  "不支持的文件类型，请上传 PDF、DOCX 或图片文件",
		}, nil
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		return ginx.Result{
			Code: errs.ResumeInternalServerError,
			Msg:  "读取文件失败",
		}, err
	}

	if len(fileData) > 10<<20 {
		return ginx.Result{
			Code: errs.ResumeFileTooLarge,
			Msg:  "文件大小不能超过10MB",
		}, nil
	}

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
