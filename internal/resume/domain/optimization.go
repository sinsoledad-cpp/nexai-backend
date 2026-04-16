package domain

// OptimizationResult 简历优化结果，包含诊断、STAR改写和JD匹配分析
type OptimizationResult struct {
	ResumeID       int64         `json:"resumeId"`       // 简历ID
	TargetPosition string        `json:"targetPosition"` // 目标岗位
	Diagnoses      []Diagnosis   `json:"diagnoses"`      // 缺陷诊断列表
	StarRewrites   []StarRewrite `json:"starRewrites"`   // STAR法则改写建议列表
	JdMatch        JdMatchResult `json:"jdMatch"`        // JD匹配结果
}

// Diagnosis 简历缺陷诊断
type Diagnosis struct {
	Target     string `json:"target"`     // 诊断目标（简历中的具体位置或内容）
	Issue      string `json:"issue"`      // 发现的问题
	Severity   string `json:"severity"`   // 严重程度（high/medium/low）
	Suggestion string `json:"suggestion"` // 改进建议
	Type       string `json:"type"`       // 问题类型（如：completeness/professionalism/quantification/formatting）
}

// StarRewrite 基于STAR法则的简历内容改写建议
type StarRewrite struct {
	Original  string `json:"original"`  // 原始内容
	Rewritten string `json:"rewritten"` // 改写后的内容
	Section   string `json:"section"`   // 所属章节（如：工作经历、项目经历）
}

// JdMatchResult 简历与目标职位描述的匹配分析结果
type JdMatchResult struct {
	MatchScore    int             `json:"matchScore"`    // 匹配分数（0-100）
	MatchedSkills []string        `json:"matchedSkills"` // 已匹配技能列表
	MissingSkills []string        `json:"missingSkills"` // 缺失技能列表
	GapAnalysis   []GapSuggestion `json:"gapAnalysis"`   // 差距分析与建议列表
}

// GapSuggestion 技能差距分析与改进建议
type GapSuggestion struct {
	Skill      string `json:"skill"`      // 技能名称
	Importance string `json:"importance"` // 重要程度（required/preferred/bonus）
	Suggestion string `json:"suggestion"` // 获取该技能的建议
	Type       string `json:"type"`       // 技能类型（如：technical/soft/domain）
}
