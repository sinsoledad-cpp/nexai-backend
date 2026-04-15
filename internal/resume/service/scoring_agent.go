package service

import (
	"context"
	"encoding/json"
	"fmt"
	"nexai-backend/internal/resume/domain"
	"nexai-backend/pkg/logger"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type ScoringWorkflow struct {
	l         logger.Logger
	chatModel model.BaseChatModel
	runnable  compose.Runnable[*scoringInput, *domain.ScoreResult]
}

type scoringInput struct {
	ParsedResume   string
	TargetPosition string
}

type llmScoreOutput struct {
	Completeness    domain.DimensionScore `json:"completeness"`
	Professionalism domain.DimensionScore `json:"professionalism"`
	Quantification  domain.DimensionScore `json:"quantification"`
	Format          domain.DimensionScore `json:"format"`
	Recommendations []string              `json:"recommendations"`
}

func NewScoringWorkflow(l logger.Logger, chatModel model.BaseChatModel) (*ScoringWorkflow, error) {
	sw := &ScoringWorkflow{
		l:         l,
		chatModel: chatModel,
	}

	wf := compose.NewWorkflow[*scoringInput, *domain.ScoreResult]()

	wf.AddLambdaNode("llm_score", compose.InvokableLambda(sw.llmScoreNode)).
		AddInput(compose.START)
	wf.AddLambdaNode("aggregate", compose.InvokableLambda(sw.aggregateNode)).
		AddInput("llm_score")
	wf.End().AddInput("aggregate")

	runnable, err := wf.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译评分工作流失败: %w", err)
	}

	sw.runnable = runnable
	return sw, nil
}

func (sw *ScoringWorkflow) Evaluate(ctx context.Context, parsedData string, targetPosition string) (domain.ScoreResult, error) {
	result, err := sw.runnable.Invoke(ctx, &scoringInput{
		ParsedResume:   parsedData,
		TargetPosition: targetPosition,
	})
	if err != nil {
		return domain.ScoreResult{}, fmt.Errorf("评分工作流执行失败: %w", err)
	}
	return *result, nil
}

func (sw *ScoringWorkflow) llmScoreNode(ctx context.Context, input *scoringInput) (*llmScoreOutput, error) {
	toolInfo := &schema.ToolInfo{
		Name: "score_resume",
		Desc: "对简历进行多维度评分",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"completeness": {
				Type: schema.Object,
				Desc: "完整度评分，包含score(0-100整数)和reasons(评分理由字符串数组)",
			},
			"professionalism": {
				Type: schema.Object,
				Desc: "专业度评分，包含score(0-100整数)和reasons(评分理由字符串数组)",
			},
			"quantification": {
				Type: schema.Object,
				Desc: "量化度评分，包含score(0-100整数)和reasons(评分理由字符串数组)",
			},
			"format": {
				Type: schema.Object,
				Desc: "排版视觉评分，包含score(0-100整数)和reasons(评分理由字符串数组)",
			},
			"recommendations": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "改进建议列表",
			},
		}),
	}

	positionDesc := "未指定意向岗位"
	if input.TargetPosition != "" {
		positionDesc = input.TargetPosition
	}

	systemPrompt := fmt.Sprintf(`你是一位专业的简历评估专家，能够对任何行业的简历进行客观、专业的评分。

你需要从以下四个维度对简历进行评分，每个维度0-100分：

1. **完整度**（Completeness）：评估简历信息的完整性
   - 个人信息是否齐全（姓名、联系方式等）
   - 教育/工作经历是否完整
   - 是否有个人简介/优势描述
   - 技能信息是否列出

2. **专业度**（Professionalism）：评估简历的专业表达水平
   - 是否使用了该行业的专业术语和规范表达
   - 工作描述是否体现了专业能力和深度
   - 是否与意向岗位「%s」的要求匹配
   - 用词是否主动、有力（如"主导""负责""推动"等），而非被动、弱化（如"参与""协助""帮忙"等）

3. **量化度**（Quantification）：评估简历中量化数据的使用情况
   - 工作成果是否有具体数字支撑（如百分比、金额、数量、时间等）
   - 是否使用了STAR法则（情境-任务-行动-结果）描述
   - 量化数据是否具有说服力

4. **排版视觉**（Format）：评估简历的结构和格式
   - 各模块是否有清晰的时间线
   - 信息层次是否分明
   - 各模块内容是否充实

注意事项：
- 你需要评估各种行业的简历，包括但不限于IT、金融、医疗、教育、法律、制造业等
- 专业度评估应基于该行业的通用标准，而非仅限于某个特定行业
- 岗位匹配度评估应考虑该岗位的行业特点
- 评分理由必须具体、有针对性，指出具体的优点或不足
- 改进建议应切实可行，帮助求职者提升简历质量

你必须调用 score_resume 函数来返回评分结果。`, positionDesc)

	userPrompt := fmt.Sprintf(`请对以下简历进行多维度评分，意向岗位为「%s」。

简历数据：
%s

请调用 score_resume 函数返回评分结果。每个维度的score为0-100的整数，reasons为具体的评分理由。`, positionDesc, input.ParsedResume)

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	resp, err := sw.chatModel.Generate(ctx, messages, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		sw.l.Error("LLM评分调用失败", logger.Error(err))
		return sw.fallbackScore(), nil
	}

	for _, toolCall := range resp.ToolCalls {
		if toolCall.Function.Name == "score_resume" {
			var result llmScoreOutput
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &result); err != nil {
				sw.l.Error("解析LLM评分结果失败", logger.Error(err))
				return sw.fallbackScore(), nil
			}
			sw.normalizeScores(&result)
			return &result, nil
		}
	}

	if resp.Content != "" {
		return sw.parseScoreFromContent(resp.Content), nil
	}

	return sw.fallbackScore(), nil
}

func (sw *ScoringWorkflow) aggregateNode(ctx context.Context, input *llmScoreOutput) (*domain.ScoreResult, error) {
	overallScore := sw.calculateOverall(input)
	return &domain.ScoreResult{
		OverallScore: overallScore,
		Dimensions: domain.ScoreDimensions{
			Completeness:    input.Completeness,
			Professionalism: input.Professionalism,
			Quantification:  input.Quantification,
			Format:          input.Format,
		},
		Recommendations: input.Recommendations,
	}, nil
}

func (sw *ScoringWorkflow) calculateOverall(input *llmScoreOutput) int {
	score := float64(input.Completeness.Score)*0.25 +
		float64(input.Professionalism.Score)*0.30 +
		float64(input.Quantification.Score)*0.25 +
		float64(input.Format.Score)*0.20
	return int(score)
}

func (sw *ScoringWorkflow) normalizeScores(result *llmScoreOutput) {
	clamp := func(score int) int {
		if score < 0 {
			return 0
		}
		if score > 100 {
			return 100
		}
		return score
	}

	result.Completeness.Score = clamp(result.Completeness.Score)
	result.Professionalism.Score = clamp(result.Professionalism.Score)
	result.Quantification.Score = clamp(result.Quantification.Score)
	result.Format.Score = clamp(result.Format.Score)

	if len(result.Completeness.Reasons) == 0 {
		result.Completeness.Reasons = []string{"完整度评估完成"}
	}
	if len(result.Professionalism.Reasons) == 0 {
		result.Professionalism.Reasons = []string{"专业度评估完成"}
	}
	if len(result.Quantification.Reasons) == 0 {
		result.Quantification.Reasons = []string{"量化度评估完成"}
	}
	if len(result.Format.Reasons) == 0 {
		result.Format.Reasons = []string{"格式评估完成"}
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = []string{"简历质量良好，继续保持"}
	}
}

func (sw *ScoringWorkflow) fallbackScore() *llmScoreOutput {
	return &llmScoreOutput{
		Completeness: domain.DimensionScore{
			Score:   50,
			Reasons: []string{"评分服务暂时不可用，返回默认分数"},
		},
		Professionalism: domain.DimensionScore{
			Score:   50,
			Reasons: []string{"评分服务暂时不可用，返回默认分数"},
		},
		Quantification: domain.DimensionScore{
			Score:   50,
			Reasons: []string{"评分服务暂时不可用，返回默认分数"},
		},
		Format: domain.DimensionScore{
			Score:   50,
			Reasons: []string{"评分服务暂时不可用，返回默认分数"},
		},
		Recommendations: []string{"评分服务暂时不可用，请稍后重试"},
	}
}

func (sw *ScoringWorkflow) parseScoreFromContent(content string) *llmScoreOutput {
	result := sw.fallbackScore()
	if strings.Contains(content, "{") {
		var parsed llmScoreOutput
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start < end {
			if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err == nil {
				sw.normalizeScores(&parsed)
				return &parsed
			}
		}
	}
	return result
}
