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

type OptimizationWorkflow struct {
	l         logger.Logger
	chatModel model.BaseChatModel
	runnable  compose.Runnable[*optimizationInput, *domain.OptimizationResult]
}

type optimizationInput struct {
	ParsedResume   string
	TargetPosition string
	JD             string
}

type llmOptimizationOutput struct {
	Diagnoses    []diagnosisItem   `json:"diagnoses"`
	StarRewrites []starRewriteItem `json:"starRewrites"`
	JdMatch      jdMatchOutput     `json:"jdMatch"`
}

type diagnosisItem struct {
	Target     string `json:"target"`
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion"`
	Type       string `json:"type"`
}

type starRewriteItem struct {
	Original  string `json:"original"`
	Rewritten string `json:"rewritten"`
	Section   string `json:"section"`
}

type jdMatchOutput struct {
	MatchScore    int                 `json:"matchScore"`
	MatchedSkills []string            `json:"matchedSkills"`
	MissingSkills []string            `json:"missingSkills"`
	GapAnalysis   []gapSuggestionItem `json:"gapAnalysis"`
}

type gapSuggestionItem struct {
	Skill      string `json:"skill"`
	Importance string `json:"importance"`
	Suggestion string `json:"suggestion"`
	Type       string `json:"type"`
}

func NewOptimizationWorkflow(l logger.Logger, chatModel model.BaseChatModel) (*OptimizationWorkflow, error) {
	ow := &OptimizationWorkflow{
		l:         l,
		chatModel: chatModel,
	}

	wf := compose.NewWorkflow[*optimizationInput, *domain.OptimizationResult]()

	wf.AddLambdaNode("llm_optimize", compose.InvokableLambda(ow.llmOptimizeNode)).
		AddInput(compose.START)
	wf.AddLambdaNode("normalize", compose.InvokableLambda(ow.normalizeNode)).
		AddInput("llm_optimize")
	wf.End().AddInput("normalize")

	runnable, err := wf.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译优化工作流失败: %w", err)
	}

	ow.runnable = runnable
	return ow, nil
}

func (ow *OptimizationWorkflow) Evaluate(ctx context.Context, parsedData string, targetPosition string, jd string) (domain.OptimizationResult, error) {
	result, err := ow.runnable.Invoke(ctx, &optimizationInput{
		ParsedResume:   parsedData,
		TargetPosition: targetPosition,
		JD:             jd,
	})
	if err != nil {
		return domain.OptimizationResult{}, fmt.Errorf("优化工作流执行失败: %w", err)
	}
	return *result, nil
}

func (ow *OptimizationWorkflow) llmOptimizeNode(ctx context.Context, input *optimizationInput) (*llmOptimizationOutput, error) {
	toolInfo := &schema.ToolInfo{
		Name: "optimize_resume",
		Desc: "对简历进行优化分析",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"diagnoses": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "缺陷诊断列表，每项包含target(问题所在模块)、issue(问题描述)、severity(high/medium/low)、suggestion(修改建议)、type(add/modify/delete)",
			},
			"starRewrites": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "STAR改写列表，每项包含original(原文)、rewritten(STAR法则改写后的文本)、section(所属模块workExperience/projects)",
			},
			"jdMatch": {
				Type: schema.Object,
				Desc: "JD匹配分析，包含matchScore(0-100匹配度)、matchedSkills(已匹配技能数组)、missingSkills(缺失技能数组)、gapAnalysis(间隙分析数组，每项含skill/importance/suggestion/type)",
			},
		}),
	}

	jdSection := "未提供目标职位描述(JD)"
	if input.JD != "" {
		jdSection = input.JD
	}

	systemPrompt := fmt.Sprintf(`你是一位资深的简历优化顾问，擅长将简历中发现的问题转化为可执行的优化操作。

你需要完成以下三个任务：

## 任务一：缺陷诊断
识别简历中的问题，包括但不限于：
- 描述过于平淡、缺乏行动力（如"参与了项目"而非"主导了项目"）
- 缺乏核心关键词和行业术语
- 段落过长、信息密度低
- 缺少量化数据支撑
- 时间线不连续或有空白期
- 个人信息不完整
- 技能描述过于笼统

每条诊断必须包含：
- target: 问题所在的模块（如personalInfo/workExperience/projects/skills/education）
- issue: 具体问题描述
- severity: 严重程度（high/medium/low）
- suggestion: 具体修改建议
- type: 建议类型（add-需要增加内容/modify-需要修改内容/delete-需要删除内容）

## 任务二：STAR改写
为每一条工作经历和项目经验提供基于STAR法则的改写建议：
- S(Situation): 描述当时的情境或背景
- T(Task): 说明需要完成的任务或目标
- A(Action): 详细描述采取的行动和方法
- R(Result): 量化呈现达成的结果和影响

改写要求：
- 使用主动、有力的动词开头（如"主导""设计""优化""推动"）
- 尽量包含量化数据（百分比、金额、时间缩短等）
- 保持简洁，每条不超过3句话
- original为原文，rewritten为改写后的文本，section为workExperience或projects

## 任务三：JD匹配度分析
根据目标职位描述，分析简历与JD的匹配情况：
- matchScore: 匹配度评分（0-100）
- matchedSkills: 简历中已具备的JD要求技能
- missingSkills: 简历中缺失的JD要求技能
- gapAnalysis: 间隙分析，每项包含：
  - skill: 缺失的技能/能力
  - importance: 重要程度（high/medium/low）
  - suggestion: 如何在简历中补充该技能的建议
  - type: 建议类型（add/modify）

意向岗位为「%s」，请基于该岗位的行业特点进行分析。

你必须调用 optimize_resume 函数来返回优化分析结果。`, input.TargetPosition)

	userPrompt := fmt.Sprintf(`请对以下简历进行优化分析，意向岗位为「%s」。

目标职位描述(JD)：
%s

简历数据：
%s

请调用 optimize_resume 函数返回结果。确保：
1. diagnoses至少包含3条诊断，最多10条
2. starRewrites为每条工作经历和项目都提供改写建议
3. jdMatch的gapAnalysis至少包含2条分析`, input.TargetPosition, jdSection, input.ParsedResume)

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	resp, err := ow.chatModel.Generate(ctx, messages, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		ow.l.Error("LLM优化调用失败", logger.Error(err))
		return ow.fallbackOptimization(), nil
	}

	for _, toolCall := range resp.ToolCalls {
		if toolCall.Function.Name == "optimize_resume" {
			var result llmOptimizationOutput
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &result); err != nil {
				ow.l.Error("解析LLM优化结果失败", logger.Error(err))
				return ow.fallbackOptimization(), nil
			}
			return &result, nil
		}
	}

	if resp.Content != "" {
		return ow.parseOptimizationFromContent(resp.Content), nil
	}

	return ow.fallbackOptimization(), nil
}

func (ow *OptimizationWorkflow) normalizeNode(ctx context.Context, input *llmOptimizationOutput) (*domain.OptimizationResult, error) {
	diagnoses := make([]domain.Diagnosis, 0, len(input.Diagnoses))
	for _, d := range input.Diagnoses {
		severity := d.Severity
		if severity != "high" && severity != "medium" && severity != "low" {
			severity = "medium"
		}
		typ := d.Type
		if typ != "add" && typ != "modify" && typ != "delete" {
			typ = "modify"
		}
		diagnoses = append(diagnoses, domain.Diagnosis{
			Target:     d.Target,
			Issue:      d.Issue,
			Severity:   severity,
			Suggestion: d.Suggestion,
			Type:       typ,
		})
	}

	starRewrites := make([]domain.StarRewrite, 0, len(input.StarRewrites))
	for _, r := range input.StarRewrites {
		section := r.Section
		if section == "" {
			section = "workExperience"
		}
		starRewrites = append(starRewrites, domain.StarRewrite{
			Original:  r.Original,
			Rewritten: r.Rewritten,
			Section:   section,
		})
	}

	matchedSkills := input.JdMatch.MatchedSkills
	if matchedSkills == nil {
		matchedSkills = []string{}
	}
	missingSkills := input.JdMatch.MissingSkills
	if missingSkills == nil {
		missingSkills = []string{}
	}

	gapAnalysis := make([]domain.GapSuggestion, 0, len(input.JdMatch.GapAnalysis))
	for _, g := range input.JdMatch.GapAnalysis {
		importance := g.Importance
		if importance != "high" && importance != "medium" && importance != "low" {
			importance = "medium"
		}
		typ := g.Type
		if typ != "add" && typ != "modify" {
			typ = "add"
		}
		gapAnalysis = append(gapAnalysis, domain.GapSuggestion{
			Skill:      g.Skill,
			Importance: importance,
			Suggestion: g.Suggestion,
			Type:       typ,
		})
	}

	matchScore := input.JdMatch.MatchScore
	if matchScore < 0 {
		matchScore = 0
	}
	if matchScore > 100 {
		matchScore = 100
	}

	return &domain.OptimizationResult{
		Diagnoses:    diagnoses,
		StarRewrites: starRewrites,
		JdMatch: domain.JdMatchResult{
			MatchScore:    matchScore,
			MatchedSkills: matchedSkills,
			MissingSkills: missingSkills,
			GapAnalysis:   gapAnalysis,
		},
	}, nil
}

func (ow *OptimizationWorkflow) fallbackOptimization() *llmOptimizationOutput {
	return &llmOptimizationOutput{
		Diagnoses: []diagnosisItem{
			{
				Target:     "general",
				Issue:      "优化服务暂时不可用",
				Severity:   "medium",
				Suggestion: "请稍后重试",
				Type:       "modify",
			},
		},
		StarRewrites: []starRewriteItem{},
		JdMatch: jdMatchOutput{
			MatchScore:    50,
			MatchedSkills: []string{},
			MissingSkills: []string{},
			GapAnalysis:   []gapSuggestionItem{},
		},
	}
}

func (ow *OptimizationWorkflow) parseOptimizationFromContent(content string) *llmOptimizationOutput {
	result := ow.fallbackOptimization()
	if strings.Contains(content, "{") {
		var parsed llmOptimizationOutput
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start < end {
			if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err == nil {
				return &parsed
			}
		}
	}
	return result
}
