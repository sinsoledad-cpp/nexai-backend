package domain

import "time"

type Resume struct {
	ID       int64
	UserID   int64
	FileName string
	FileURL  string
	FileType string
	RawText  string
	Parsed   ParsedResume
	Status   ResumeStatus
	Ctime    time.Time
	Utime    time.Time
}

type ResumeStatus int

const (
	StatusUploaded ResumeStatus = iota
	StatusParsing
	StatusParsed
	StatusParseFailed
	StatusScored
)

type ParsedResume struct {
	PersonalInfo   PersonalInfo     `json:"personalInfo"`
	Education      []Education      `json:"education"`
	WorkExperience []WorkExperience `json:"workExperience"`
	Projects       []Project        `json:"projects"`
	Skills         []string         `json:"skills"`
}

type PersonalInfo struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
	Summary string `json:"summary"`
}

type Education struct {
	School    string `json:"school"`
	Degree    string `json:"degree"`
	Major     string `json:"major"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type WorkExperience struct {
	Company     string `json:"company"`
	Position    string `json:"position"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Description string `json:"description"`
}

type Project struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Description string `json:"description"`
}

type ScoreResult struct {
	ResumeID        int64           `json:"resumeId"`
	OverallScore    int             `json:"overallScore"`
	Dimensions      ScoreDimensions `json:"dimensions"`
	Recommendations []string        `json:"recommendations"`
	TargetPosition  string          `json:"targetPosition"`
}

type ScoreDimensions struct {
	Completeness    DimensionScore `json:"completeness"`
	Professionalism DimensionScore `json:"professionalism"`
	Quantification  DimensionScore `json:"quantification"`
	Format          DimensionScore `json:"format"`
}

type DimensionScore struct {
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}
