package service

import (
	"context"
	"encoding/json"
	"fmt"
	"nexai-backend/internal/resume/domain"
	"nexai-backend/pkg/logger"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/model"
)

type ScoringAgent struct {
	l         logger.Logger
	chatModel model.BaseChatModel
}

func NewScoringAgent(l logger.Logger, chatModel model.BaseChatModel) *ScoringAgent {
	return &ScoringAgent{
		l:         l,
		chatModel: chatModel,
	}
}

func (sa *ScoringAgent) Evaluate(ctx context.Context, parsedData string, targetPosition string) (domain.ScoreResult, error) {
	var parsed domain.ParsedResume
	if err := json.Unmarshal([]byte(parsedData), &parsed); err != nil {
		return domain.ScoreResult{}, fmt.Errorf("解析简历数据失败: %w", err)
	}

	completeness := sa.evaluateCompleteness(parsed)
	professionalism := sa.evaluateProfessionalism(ctx, parsed, targetPosition)
	quantification := sa.evaluateQuantification(parsed)
	formatScore := sa.evaluateFormat(parsed)

	weights := sa.getPositionWeights(targetPosition)
	overallScore := sa.calculateOverall(weights, completeness, professionalism, quantification, formatScore)

	recommendations := sa.generateRecommendations(parsed, completeness, professionalism, quantification, formatScore, targetPosition)

	return domain.ScoreResult{
		OverallScore: overallScore,
		Dimensions: domain.ScoreDimensions{
			Completeness:    completeness,
			Professionalism: professionalism,
			Quantification:  quantification,
			Format:          formatScore,
		},
		Recommendations: recommendations,
	}, nil
}

func (sa *ScoringAgent) evaluateCompleteness(parsed domain.ParsedResume) domain.DimensionScore {
	score := 100
	var reasons []string

	if parsed.PersonalInfo.Name == "" || parsed.PersonalInfo.Name == "未知" {
		score -= 15
		reasons = append(reasons, "缺少姓名信息")
	}
	if parsed.PersonalInfo.Phone == "" {
		score -= 15
		reasons = append(reasons, "缺少联系电话")
	}
	if parsed.PersonalInfo.Email == "" {
		score -= 10
		reasons = append(reasons, "缺少邮箱地址")
	}
	if len(parsed.Education) == 0 {
		score -= 20
		reasons = append(reasons, "缺少教育背景")
	}
	if len(parsed.WorkExperience) == 0 {
		score -= 20
		reasons = append(reasons, "缺少工作经历")
	}
	if len(parsed.Skills) == 0 {
		score -= 10
		reasons = append(reasons, "缺少技能信息")
	}
	if parsed.PersonalInfo.Summary == "" {
		score -= 10
		reasons = append(reasons, "缺少个人简介/优势描述")
	}

	if score < 0 {
		score = 0
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "简历信息完整，各关键模块齐全")
	}

	return domain.DimensionScore{
		Score:   score,
		Reasons: reasons,
	}
}

func (sa *ScoringAgent) evaluateProfessionalism(ctx context.Context, parsed domain.ParsedResume, targetPosition string) domain.DimensionScore {
	score := 60
	var reasons []string

	professionalKeywords := []string{
		"主导", "负责", "设计", "实现", "优化", "重构", "落地",
		"架构", "方案", "规划", "推动", "交付", "提升", "降低",
		"搭建", "构建", "解决", "攻克", "突破",
	}

	weakKeywords := []string{
		"参与", "协助", "帮忙", "了解", "知道", "简单",
	}

	professionalCount := 0
	weakCount := 0
	allText := sa.concatAllText(parsed)

	for _, kw := range professionalKeywords {
		count := strings.Count(allText, kw)
		professionalCount += count
	}

	for _, kw := range weakKeywords {
		count := strings.Count(allText, kw)
		weakCount += count
	}

	if professionalCount >= 5 {
		score += 25
		reasons = append(reasons, "使用了大量专业行业术语，表达专业")
	} else if professionalCount >= 2 {
		score += 15
		reasons = append(reasons, "使用了一定数量的专业术语")
	} else {
		reasons = append(reasons, "专业术语使用较少，建议使用更专业的表述")
	}

	if weakCount > 3 {
		score -= 10
		reasons = append(reasons, "使用了较多弱化词（如'参与'、'协助'），建议替换为更主动的表述")
	}

	if targetPosition != "" {
		positionKeywords := sa.getPositionKeywords(targetPosition)
		matchCount := 0
		for _, kw := range positionKeywords {
			if strings.Contains(allText, kw) {
				matchCount++
			}
		}
		if matchCount >= 3 {
			score += 15
			reasons = append(reasons, fmt.Sprintf("与%s岗位关键词高度匹配", targetPosition))
		} else if matchCount >= 1 {
			score += 5
			reasons = append(reasons, fmt.Sprintf("与%s岗位部分关键词匹配", targetPosition))
		} else {
			reasons = append(reasons, fmt.Sprintf("与%s岗位关键词匹配度较低", targetPosition))
		}
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "专业度评估完成")
	}

	return domain.DimensionScore{
		Score:   score,
		Reasons: reasons,
	}
}

func (sa *ScoringAgent) evaluateQuantification(parsed domain.ParsedResume) domain.DimensionScore {
	score := 50
	var reasons []string

	quantPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\d+%`),
		regexp.MustCompile(`\d+万`),
		regexp.MustCompile(`\d+倍`),
		regexp.MustCompile(`\d+人`),
		regexp.MustCompile(`\d+个`),
		regexp.MustCompile(`\d+次`),
		regexp.MustCompile(`\d+天`),
		regexp.MustCompile(`\d+小时`),
		regexp.MustCompile(`\d+分钟`),
		regexp.MustCompile(`\d+ms`),
		regexp.MustCompile(`\d+QPS`),
		regexp.MustCompile(`\d+TPS`),
		regexp.MustCompile(`提升了?\s*\d+`),
		regexp.MustCompile(`降低\s*\d+`),
		regexp.MustCompile(`减少\s*\d+`),
		regexp.MustCompile(`增加\s*\d+`),
		regexp.MustCompile(`节省\s*\d+`),
	}

	allText := sa.concatAllText(parsed)
	quantCount := 0
	for _, pattern := range quantPatterns {
		matches := pattern.FindAllString(allText, -1)
		quantCount += len(matches)
	}

	if quantCount >= 5 {
		score += 40
		reasons = append(reasons, "工作描述中包含丰富的量化数据，非常有说服力")
	} else if quantCount >= 3 {
		score += 25
		reasons = append(reasons, "工作描述中包含一定的量化数据")
	} else if quantCount >= 1 {
		score += 10
		reasons = append(reasons, "工作描述中量化数据较少，建议补充更多具体数字")
	} else {
		reasons = append(reasons, "工作描述中缺乏量化数据，建议使用STAR法则补充具体成果数据")
	}

	for _, work := range parsed.WorkExperience {
		if work.Description != "" && len(work.Description) > 50 {
			score += 3
		}
	}

	for _, proj := range parsed.Projects {
		if proj.Description != "" && len(proj.Description) > 50 {
			score += 3
		}
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "量化度评估完成")
	}

	return domain.DimensionScore{
		Score:   score,
		Reasons: reasons,
	}
}

func (sa *ScoringAgent) evaluateFormat(parsed domain.ParsedResume) domain.DimensionScore {
	score := 70
	var reasons []string

	if len(parsed.WorkExperience) > 0 {
		hasDateRange := false
		for _, work := range parsed.WorkExperience {
			if work.StartDate != "" || work.EndDate != "" {
				hasDateRange = true
				break
			}
		}
		if hasDateRange {
			score += 10
		} else {
			score -= 10
			reasons = append(reasons, "工作经历缺少时间信息")
		}
	}

	if len(parsed.Education) > 0 {
		hasDegree := false
		for _, edu := range parsed.Education {
			if edu.Degree != "" {
				hasDegree = true
				break
			}
		}
		if hasDegree {
			score += 5
		} else {
			score -= 5
			reasons = append(reasons, "教育背景缺少学位信息")
		}
	}

	if len(parsed.Skills) >= 5 {
		score += 10
	} else if len(parsed.Skills) >= 3 {
		score += 5
	} else {
		reasons = append(reasons, "技能列表较少，建议补充更多相关技能")
	}

	if len(parsed.WorkExperience) >= 2 {
		score += 5
	}

	if len(parsed.Projects) >= 1 {
		score += 5
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "简历格式规范，结构清晰")
	}

	return domain.DimensionScore{
		Score:   score,
		Reasons: reasons,
	}
}

type positionWeight struct {
	completeness    float64
	professionalism float64
	quantification  float64
	format          float64
}

func (sa *ScoringAgent) getPositionWeights(position string) positionWeight {
	position = strings.ToLower(position)

	switch {
	case strings.Contains(position, "前端") || strings.Contains(position, "frontend"):
		return positionWeight{0.25, 0.30, 0.25, 0.20}
	case strings.Contains(position, "后端") || strings.Contains(position, "backend"):
		return positionWeight{0.20, 0.35, 0.30, 0.15}
	case strings.Contains(position, "ui") || strings.Contains(position, "设计") || strings.Contains(position, "design"):
		return positionWeight{0.20, 0.25, 0.15, 0.40}
	case strings.Contains(position, "产品") || strings.Contains(position, "product"):
		return positionWeight{0.25, 0.30, 0.25, 0.20}
	case strings.Contains(position, "数据") || strings.Contains(position, "data"):
		return positionWeight{0.20, 0.30, 0.35, 0.15}
	default:
		return positionWeight{0.25, 0.30, 0.25, 0.20}
	}
}

func (sa *ScoringAgent) calculateOverall(weights positionWeight, completeness, professionalism, quantification, format domain.DimensionScore) int {
	score := float64(completeness.Score)*weights.completeness +
		float64(professionalism.Score)*weights.professionalism +
		float64(quantification.Score)*weights.quantification +
		float64(format.Score)*weights.format

	return int(score)
}

func (sa *ScoringAgent) getPositionKeywords(position string) []string {
	position = strings.ToLower(position)

	keywords := map[string][]string{
		"前端": {"Vue", "React", "TypeScript", "JavaScript", "CSS", "HTML", "Webpack", "Vite", "组件化", "响应式"},
		"后端": {"Go", "Java", "Python", "微服务", "分布式", "高并发", "数据库", "Redis", "Kafka", "API"},
		"ui": {"Figma", "Sketch", "交互设计", "视觉设计", "用户体验", "设计系统"},
		"设计": {"Figma", "Sketch", "交互设计", "视觉设计", "用户体验", "设计系统"},
		"产品": {"需求分析", "用户调研", "竞品分析", "PRD", "原型设计", "数据驱动"},
		"数据": {"SQL", "Python", "Spark", "Hadoop", "机器学习", "数据分析", "数据仓库"},
	}

	for key, words := range keywords {
		if strings.Contains(position, key) {
			return words
		}
	}

	return []string{}
}

func (sa *ScoringAgent) generateRecommendations(parsed domain.ParsedResume, completeness, professionalism, quantification, format domain.DimensionScore, targetPosition string) []string {
	var recs []string

	if completeness.Score < 70 {
		recs = append(recs, "建议补充简历中的缺失信息，确保联系方式、教育背景和工作经历完整")
	}

	if professionalism.Score < 70 {
		recs = append(recs, "建议使用更专业的行业术语，如'主导'、'落地'、'重构'等，替代弱化词")
	}

	if quantification.Score < 70 {
		recs = append(recs, "建议在工作描述中使用STAR法则，补充具体的量化数据（如百分比、金额、时间缩短等）")
	}

	if format.Score < 70 {
		recs = append(recs, "建议优化简历格式，确保时间线完整、技能列表充实")
	}

	if targetPosition != "" {
		keywords := sa.getPositionKeywords(targetPosition)
		if len(keywords) > 0 {
			allText := sa.concatAllText(parsed)
			matchCount := 0
			for _, kw := range keywords {
				if strings.Contains(allText, kw) {
					matchCount++
				}
			}
			if matchCount < 3 {
				recs = append(recs, fmt.Sprintf("建议在简历中突出与%s岗位相关的技能和经验", targetPosition))
			}
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "简历质量良好，继续保持")
	}

	return recs
}

func (sa *ScoringAgent) concatAllText(parsed domain.ParsedResume) string {
	var sb strings.Builder
	sb.WriteString(parsed.PersonalInfo.Summary)
	sb.WriteString(" ")
	for _, work := range parsed.WorkExperience {
		sb.WriteString(work.Description)
		sb.WriteString(" ")
	}
	for _, proj := range parsed.Projects {
		sb.WriteString(proj.Description)
		sb.WriteString(" ")
	}
	sb.WriteString(strings.Join(parsed.Skills, " "))
	return sb.String()
}
