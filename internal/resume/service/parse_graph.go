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
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type ParseWorkflow struct {
	l         logger.Logger
	chatModel model.BaseChatModel
	extractor *TextExtractor
	runnable  compose.Runnable[*parseInput, *domain.ParsedResume]
}

type parseInput struct {
	RawText string
}

type cleanedText struct {
	Content string
}

func NewParseWorkflow(l logger.Logger, chatModel model.BaseChatModel) (*ParseWorkflow, error) {
	pw := &ParseWorkflow{
		l:         l,
		chatModel: chatModel,
		extractor: NewTextExtractor(),
	}

	wf := compose.NewWorkflow[*parseInput, *domain.ParsedResume]()

	wf.AddLambdaNode("text_clean", compose.InvokableLambda(pw.textCleanNode)).
		AddInput(compose.START)
	wf.AddLambdaNode("llm_extract", compose.InvokableLambda(pw.llmExtractNode)).
		AddInput("text_clean")
	wf.AddLambdaNode("schema_validate", compose.InvokableLambda(pw.schemaValidateNode)).
		AddInput("llm_extract")
	wf.End().AddInput("schema_validate")

	runnable, err := wf.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译解析工作流失败: %w", err)
	}

	pw.runnable = runnable
	return pw, nil
}

func (pw *ParseWorkflow) Run(ctx context.Context, rawText string) (domain.ParsedResume, error) {
	result, err := pw.runnable.Invoke(ctx, &parseInput{RawText: rawText})
	if err != nil {
		return domain.ParsedResume{}, fmt.Errorf("解析工作流执行失败: %w", err)
	}
	return *result, nil
}

func (pw *ParseWorkflow) ExtractText(ctx context.Context, fileType string, data []byte) (string, error) {
	return pw.extractor.Extract(fileType, data)
}

func (pw *ParseWorkflow) textCleanNode(ctx context.Context, input *parseInput) (*cleanedText, error) {
	text := input.RawText

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", " ")

	re := regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")

	re = regexp.MustCompile(` {2,}`)
	text = re.ReplaceAllString(text, " ")

	text = strings.TrimSpace(text)

	pw.l.Debug("文本清洗完成", logger.Field{Key: "length", Val: len(text)})

	return &cleanedText{Content: text}, nil
}

func (pw *ParseWorkflow) llmExtractNode(ctx context.Context, input *cleanedText) (*domain.ParsedResume, error) {
	toolInfo := &schema.ToolInfo{
		Name: "extract_resume_info",
		Desc: "从简历文本中提取结构化信息",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Type: schema.String,
				Desc: "候选人姓名",
			},
			"phone": {
				Type: schema.String,
				Desc: "手机号码",
			},
			"email": {
				Type: schema.String,
				Desc: "邮箱地址",
			},
			"address": {
				Type: schema.String,
				Desc: "地址",
			},
			"summary": {
				Type: schema.String,
				Desc: "个人简介/优势",
			},
			"education": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "教育背景列表，每项包含school(学校)、degree(学位)、major(专业)、start_date(开始时间)、end_date(结束时间)",
			},
			"work_experience": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "工作经历列表，每项包含company(公司)、position(职位)、start_date(开始时间)、end_date(结束时间)、description(工作描述)",
			},
			"projects": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "项目经验列表，每项包含name(项目名)、role(角色)、start_date(开始时间)、end_date(结束时间)、description(项目描述)",
			},
			"skills": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "技能列表",
			},
		}),
	}

	systemPrompt := `你是一个专业的简历信息提取专家，能够从任何行业、任何格式的简历中精准提取结构化信息。

你的任务是从简历文本中提取以下信息：
1. 个人信息：姓名、电话、邮箱、地址、个人简介/优势
2. 教育背景：学校、学位、专业、起止时间
3. 工作经历：公司、职位、起止时间、工作描述
4. 项目经验：项目名、角色、起止时间、项目描述
5. 技能栈：所有技能关键词

注意事项：
- 你需要处理各种行业的简历，包括但不限于IT、金融、医疗、教育、法律、制造业等
- 不同行业的简历可能有不同的模块命名和结构，请灵活识别
- 工作描述和项目描述应保留原文的关键信息，不要遗漏
- 如果某个字段在简历中确实不存在，请留空字符串或空数组
- 你必须调用 extract_resume_info 函数来返回提取结果`

	userPrompt := fmt.Sprintf(`请从以下简历文本中提取结构化信息，调用 extract_resume_info 函数返回结果：

%s`, input.Content)

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	resp, err := pw.chatModel.Generate(ctx, messages, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		pw.l.Error("LLM调用失败", logger.Error(err))
		return pw.fallbackExtract(input), nil
	}

	for _, toolCall := range resp.ToolCalls {
		if toolCall.Function.Name == "extract_resume_info" {
			var result struct {
				Name      string `json:"name"`
				Phone     string `json:"phone"`
				Email     string `json:"email"`
				Address   string `json:"address"`
				Summary   string `json:"summary"`
				Education []struct {
					School    string `json:"school"`
					Degree    string `json:"degree"`
					Major     string `json:"major"`
					StartDate string `json:"start_date"`
					EndDate   string `json:"end_date"`
				} `json:"education"`
				WorkExperience []struct {
					Company     string `json:"company"`
					Position    string `json:"position"`
					StartDate   string `json:"start_date"`
					EndDate     string `json:"end_date"`
					Description string `json:"description"`
				} `json:"work_experience"`
				Projects []struct {
					Name        string `json:"name"`
					Role        string `json:"role"`
					StartDate   string `json:"start_date"`
					EndDate     string `json:"end_date"`
					Description string `json:"description"`
				} `json:"projects"`
				Skills []string `json:"skills"`
			}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &result); err != nil {
				pw.l.Error("解析LLM返回结果失败", logger.Error(err))
				return pw.fallbackExtract(input), nil
			}

			parsed := &domain.ParsedResume{
				PersonalInfo: domain.PersonalInfo{
					Name:    result.Name,
					Phone:   result.Phone,
					Email:   result.Email,
					Address: result.Address,
					Summary: result.Summary,
				},
				Skills: result.Skills,
			}

			for _, edu := range result.Education {
				parsed.Education = append(parsed.Education, domain.Education{
					School:    edu.School,
					Degree:    edu.Degree,
					Major:     edu.Major,
					StartDate: edu.StartDate,
					EndDate:   edu.EndDate,
				})
			}

			for _, work := range result.WorkExperience {
				parsed.WorkExperience = append(parsed.WorkExperience, domain.WorkExperience{
					Company:     work.Company,
					Position:    work.Position,
					StartDate:   work.StartDate,
					EndDate:     work.EndDate,
					Description: work.Description,
				})
			}

			for _, proj := range result.Projects {
				parsed.Projects = append(parsed.Projects, domain.Project{
					Name:        proj.Name,
					Role:        proj.Role,
					StartDate:   proj.StartDate,
					EndDate:     proj.EndDate,
					Description: proj.Description,
				})
			}

			return parsed, nil
		}
	}

	if resp.Content != "" {
		return pw.parseFromContent(resp.Content), nil
	}

	return pw.fallbackExtract(input), nil
}

func (pw *ParseWorkflow) schemaValidateNode(ctx context.Context, input *domain.ParsedResume) (*domain.ParsedResume, error) {
	if input.PersonalInfo.Name == "" {
		input.PersonalInfo.Name = "未知"
	}
	if len(input.Skills) == 0 {
		input.Skills = []string{}
	}
	if len(input.Education) == 0 {
		input.Education = []domain.Education{}
	}
	if len(input.WorkExperience) == 0 {
		input.WorkExperience = []domain.WorkExperience{}
	}
	if len(input.Projects) == 0 {
		input.Projects = []domain.Project{}
	}
	return input, nil
}

func (pw *ParseWorkflow) fallbackExtract(input *cleanedText) *domain.ParsedResume {
	result := &domain.ParsedResume{
		PersonalInfo: domain.PersonalInfo{
			Name:    "未知",
			Summary: input.Content,
		},
		Education:      []domain.Education{},
		WorkExperience: []domain.WorkExperience{},
		Projects:       []domain.Project{},
		Skills:         []string{},
	}
	return result
}

func (pw *ParseWorkflow) parseFromContent(content string) *domain.ParsedResume {
	result := &domain.ParsedResume{
		PersonalInfo: domain.PersonalInfo{
			Name:    "未知",
			Summary: content,
		},
		Education:      []domain.Education{},
		WorkExperience: []domain.WorkExperience{},
		Projects:       []domain.Project{},
		Skills:         []string{},
	}
	return result
}
