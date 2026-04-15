package domain

// ScoreResult 评分结果
type ScoreResult struct {
	ResumeID        int64           `json:"resumeId"`        // 简历ID
	OverallScore    int             `json:"overallScore"`    // 综合评分
	Dimensions      ScoreDimensions `json:"dimensions"`      // 各维度评分
	Recommendations []string        `json:"recommendations"` // 改进建议
	TargetPosition  string          `json:"targetPosition"`  // 意向岗位
}

// ScoreDimensions 评分维度
type ScoreDimensions struct {
	Completeness    DimensionScore `json:"completeness"`    // 完整度评分
	Professionalism DimensionScore `json:"professionalism"` // 专业度评分
	Quantification  DimensionScore `json:"quantification"`  // 量化度评分
	Format          DimensionScore `json:"format"`          // 排版视觉评分
}

// DimensionScore 维度评分
type DimensionScore struct {
	Score   int      `json:"score"`   // 分数（0-100）
	Reasons []string `json:"reasons"` // 评分理由
}
