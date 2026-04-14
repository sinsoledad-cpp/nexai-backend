package dto

type UploadResponse struct {
	FileID   int64  `json:"fileId"`
	FileName string `json:"fileName"`
	FileType string `json:"fileType"`
}

type ParseRequest struct {
	FileID int64 `json:"fileId" binding:"required"`
}

type ParseResponse struct {
	PersonalInfo   PersonalInfoDTO     `json:"personalInfo"`
	Education      []EducationDTO      `json:"education"`
	WorkExperience []WorkExperienceDTO `json:"workExperience"`
	Projects       []ProjectDTO        `json:"projects"`
	Skills         []string            `json:"skills"`
}

type PersonalInfoDTO struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
	Summary string `json:"summary"`
}

type EducationDTO struct {
	School    string `json:"school"`
	Degree    string `json:"degree"`
	Major     string `json:"major"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type WorkExperienceDTO struct {
	Company     string `json:"company"`
	Position    string `json:"position"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Description string `json:"description"`
}

type ProjectDTO struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Description string `json:"description"`
}

type CorrectRequest struct {
	FileID int64         `json:"fileId" binding:"required"`
	Parsed ParseResponse `json:"parsed" binding:"required"`
}

type ScoreRequest struct {
	FileID         int64  `json:"fileId" binding:"required"`
	TargetPosition string `json:"targetPosition" binding:"required"`
}

type ScoreResponse struct {
	ResumeID        int64              `json:"resumeId"`
	OverallScore    int                `json:"overallScore"`
	Dimensions      ScoreDimensionsDTO `json:"dimensions"`
	Recommendations []string           `json:"recommendations"`
	TargetPosition  string             `json:"targetPosition"`
}

type ScoreDimensionsDTO struct {
	Completeness    DimensionScoreDTO `json:"completeness"`
	Professionalism DimensionScoreDTO `json:"professionalism"`
	Quantification  DimensionScoreDTO `json:"quantification"`
	Format          DimensionScoreDTO `json:"format"`
}

type DimensionScoreDTO struct {
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}
