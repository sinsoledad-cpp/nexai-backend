package domain

type OptimizationResult struct {
	ResumeID       int64         `json:"resumeId"`
	TargetPosition string        `json:"targetPosition"`
	Diagnoses      []Diagnosis   `json:"diagnoses"`
	StarRewrites   []StarRewrite `json:"starRewrites"`
	JdMatch        JdMatchResult `json:"jdMatch"`
}

type Diagnosis struct {
	Target     string `json:"target"`
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion"`
	Type       string `json:"type"`
}

type StarRewrite struct {
	Original  string `json:"original"`
	Rewritten string `json:"rewritten"`
	Section   string `json:"section"`
}

type JdMatchResult struct {
	MatchScore    int             `json:"matchScore"`
	MatchedSkills []string        `json:"matchedSkills"`
	MissingSkills []string        `json:"missingSkills"`
	GapAnalysis   []GapSuggestion `json:"gapAnalysis"`
}

type GapSuggestion struct {
	Skill      string `json:"skill"`
	Importance string `json:"importance"`
	Suggestion string `json:"suggestion"`
	Type       string `json:"type"`
}
