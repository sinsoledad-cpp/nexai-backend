package dto

// UploadResponse 上传响应
type UploadResponse struct {
	FileID   int64  `json:"fileId"`   // 文件ID
	FileName string `json:"fileName"` // 文件名
	FileType string `json:"fileType"` // 文件类型
}

// ParseRequest 解析请求
type ParseRequest struct {
	FileID int64 `json:"fileId" binding:"required"` // 文件ID
}

// ParseResponse 解析响应
type ParseResponse struct {
	PersonalInfo   PersonalInfoDTO     `json:"personalInfo"`   // 个人信息
	Education      []EducationDTO      `json:"education"`      // 教育背景
	WorkExperience []WorkExperienceDTO `json:"workExperience"` // 工作经历
	Projects       []ProjectDTO        `json:"projects"`       // 项目经验
	Skills         []string            `json:"skills"`         // 技能栈
}

// PersonalInfoDTO 个人信息DTO
type PersonalInfoDTO struct {
	Name    string `json:"name"`    // 姓名
	Phone   string `json:"phone"`   // 手机号码
	Email   string `json:"email"`   // 邮箱地址
	Address string `json:"address"` // 地址
	Summary string `json:"summary"` // 个人简介/优势
}

// EducationDTO 教育背景DTO
type EducationDTO struct {
	School    string `json:"school"`    // 学校名称
	Degree    string `json:"degree"`    // 学位
	Major     string `json:"major"`     // 专业
	StartDate string `json:"startDate"` // 开始时间
	EndDate   string `json:"endDate"`   // 结束时间
}

// WorkExperienceDTO 工作经历DTO
type WorkExperienceDTO struct {
	Company     string `json:"company"`     // 公司名称
	Position    string `json:"position"`    // 职位
	StartDate   string `json:"startDate"`   // 开始时间
	EndDate     string `json:"endDate"`     // 结束时间
	Description string `json:"description"` // 工作描述
}

// ProjectDTO 项目经验DTO
type ProjectDTO struct {
	Name        string `json:"name"`        // 项目名称
	Role        string `json:"role"`        // 角色
	StartDate   string `json:"startDate"`   // 开始时间
	EndDate     string `json:"endDate"`     // 结束时间
	Description string `json:"description"` // 项目描述
}

// CorrectRequest 修正请求
type CorrectRequest struct {
	FileID int64         `json:"fileId" binding:"required"` // 文件ID
	Parsed ParseResponse `json:"parsed" binding:"required"` // 修正后的解析数据
}

// ScoreRequest 评分请求
type ScoreRequest struct {
	FileID         int64  `json:"fileId" binding:"required"`         // 文件ID
	TargetPosition string `json:"targetPosition" binding:"required"` // 意向岗位
}

// ScoreResponse 评分响应
type ScoreResponse struct {
	ResumeID        int64              `json:"resumeId"`        // 简历ID
	OverallScore    int                `json:"overallScore"`    // 综合评分
	Dimensions      ScoreDimensionsDTO `json:"dimensions"`      // 各维度评分
	Recommendations []string           `json:"recommendations"` // 改进建议
	TargetPosition  string             `json:"targetPosition"`  // 意向岗位
}

// ScoreDimensionsDTO 评分维度DTO
type ScoreDimensionsDTO struct {
	Completeness    DimensionScoreDTO `json:"completeness"`    // 完整度评分
	Professionalism DimensionScoreDTO `json:"professionalism"` // 专业度评分
	Quantification  DimensionScoreDTO `json:"quantification"`  // 量化度评分
	Format          DimensionScoreDTO `json:"format"`          // 排版视觉评分
}

// DimensionScoreDTO 维度评分DTO
type DimensionScoreDTO struct {
	Score   int      `json:"score"`   // 分数（0-100）
	Reasons []string `json:"reasons"` // 评分理由
}

// OptimizeRequest 优化建议请求
type OptimizeRequest struct {
	FileID         int64  `json:"fileId" binding:"required"`         // 文件ID
	TargetPosition string `json:"targetPosition" binding:"required"` // 目标岗位
	JD             string `json:"jd"`                                // 职位描述（选填）
}

// OptimizeResponse 优化建议响应
type OptimizeResponse struct {
	ResumeID       int64            `json:"resumeId"`       // 简历ID
	TargetPosition string           `json:"targetPosition"` // 目标岗位
	Diagnoses      []DiagnosisDTO   `json:"diagnoses"`      // 缺陷诊断列表
	StarRewrites   []StarRewriteDTO `json:"starRewrites"`   // STAR法则改写建议列表
	JdMatch        JdMatchResultDTO `json:"jdMatch"`        // JD匹配结果
}

// DiagnosisDTO 缺陷诊断DTO
type DiagnosisDTO struct {
	Target     string `json:"target"`     // 诊断目标（简历中的具体位置或内容）
	Issue      string `json:"issue"`      // 发现的问题
	Severity   string `json:"severity"`   // 严重程度（high/medium/low）
	Suggestion string `json:"suggestion"` // 改进建议
	Type       string `json:"type"`       // 问题类型（如：completeness/professionalism/quantification/formatting）
}

// StarRewriteDTO STAR改写DTO
type StarRewriteDTO struct {
	Original  string `json:"original"`  // 原始内容
	Rewritten string `json:"rewritten"` // 改写后的内容
	Section   string `json:"section"`   // 所属章节（如：工作经历、项目经历）
}

// JdMatchResultDTO JD匹配结果DTO
type JdMatchResultDTO struct {
	MatchScore    int                `json:"matchScore"`    // 匹配分数（0-100）
	MatchedSkills []string           `json:"matchedSkills"` // 已匹配技能列表
	MissingSkills []string           `json:"missingSkills"` // 缺失技能列表
	GapAnalysis   []GapSuggestionDTO `json:"gapAnalysis"`   // 差距分析与建议列表
}

// GapSuggestionDTO 技能差距分析与改进建议DTO
type GapSuggestionDTO struct {
	Skill      string `json:"skill"`      // 技能名称
	Importance string `json:"importance"` // 重要程度（required/preferred/bonus）
	Suggestion string `json:"suggestion"` // 获取该技能的建议
	Type       string `json:"type"`       // 技能类型（如：technical/soft/domain）
}
