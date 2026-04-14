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

type ParseGraph struct {
	l         logger.Logger
	chatModel model.BaseChatModel
	runnable  compose.Runnable[*parseInput, *domain.ParsedResume]
}

type parseInput struct {
	RawText string
}

type cleanedText struct {
	Content string
}

type moduleSegments struct {
	PersonalInfo string
	Education    string
	Work         string
	Projects     string
	Skills       string
}

func NewParseGraph(l logger.Logger, chatModel model.BaseChatModel) (*ParseGraph, error) {
	pg := &ParseGraph{
		l:         l,
		chatModel: chatModel,
	}

	graph := compose.NewGraph[*parseInput, *domain.ParsedResume]()

	err := graph.AddLambdaNode("text_clean", compose.InvokableLambda(pg.textCleanNode))
	if err != nil {
		return nil, fmt.Errorf("添加文本清洗节点失败: %w", err)
	}

	err = graph.AddLambdaNode("module_split", compose.InvokableLambda(pg.moduleSplitNode))
	if err != nil {
		return nil, fmt.Errorf("添加模块切分节点失败: %w", err)
	}

	err = graph.AddLambdaNode("llm_extract", compose.InvokableLambda(pg.llmExtractNode))
	if err != nil {
		return nil, fmt.Errorf("添加LLM提取节点失败: %w", err)
	}

	err = graph.AddLambdaNode("schema_validate", compose.InvokableLambda(pg.schemaValidateNode))
	if err != nil {
		return nil, fmt.Errorf("添加Schema校验节点失败: %w", err)
	}

	err = graph.AddEdge(compose.START, "text_clean")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("text_clean", "module_split")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("module_split", "llm_extract")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("llm_extract", "schema_validate")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("schema_validate", compose.END)
	if err != nil {
		return nil, err
	}

	runnable, err := graph.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译解析图失败: %w", err)
	}

	pg.runnable = runnable
	return pg, nil
}

func (pg *ParseGraph) Run(ctx context.Context, rawText string) (domain.ParsedResume, error) {
	result, err := pg.runnable.Invoke(ctx, &parseInput{RawText: rawText})
	if err != nil {
		return domain.ParsedResume{}, fmt.Errorf("解析图执行失败: %w", err)
	}
	return *result, nil
}

func (pg *ParseGraph) ExtractText(ctx context.Context, fileType string, data []byte) (string, error) {
	switch fileType {
	case ".pdf":
		return pg.extractFromPDF(data)
	case ".docx", ".doc":
		return pg.extractFromDocx(data)
	case ".png", ".jpg", ".jpeg":
		return pg.extractFromImage(data)
	default:
		return "", ErrFileTypeUnsupported
	}
}

func (pg *ParseGraph) extractFromPDF(data []byte) (string, error) {
	return string(data), nil
}

func (pg *ParseGraph) extractFromDocx(data []byte) (string, error) {
	return string(data), nil
}

func (pg *ParseGraph) extractFromImage(data []byte) (string, error) {
	return string(data), nil
}

func (pg *ParseGraph) textCleanNode(ctx context.Context, input *parseInput) (*cleanedText, error) {
	text := input.RawText

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", " ")

	re := regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")

	re = regexp.MustCompile(` {2,}`)
	text = re.ReplaceAllString(text, " ")

	text = strings.TrimSpace(text)

	pg.l.Debug("文本清洗完成", logger.Field{Key: "length", Val: len(text)})

	return &cleanedText{Content: text}, nil
}

func (pg *ParseGraph) moduleSplitNode(ctx context.Context, input *cleanedText) (*moduleSegments, error) {
	segments := &moduleSegments{}
	text := input.Content

	sections := map[string]*string{
		"个人信息": &segments.PersonalInfo,
		"教育背景": &segments.Education,
		"工作经历": &segments.Work,
		"项目经验": &segments.Projects,
		"技能":   &segments.Skills,
	}

	sectionKeywords := map[string][]string{
		"个人信息": {"个人信息", "基本信息", "联系方式", "个人资料"},
		"教育背景": {"教育背景", "教育经历", "学历", "学术背景"},
		"工作经历": {"工作经历", "工作经验", "工作背景", "职业经历"},
		"项目经验": {"项目经验", "项目经历", "项目"},
		"技能":   {"技能", "专业技能", "技术栈", "核心技能"},
	}

	lines := strings.Split(text, "\n")
	currentSection := "个人信息"
	var sectionLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			sectionLines = append(sectionLines, line)
			continue
		}

		found := false
		for section, keywords := range sectionKeywords {
			for _, kw := range keywords {
				if strings.Contains(line, kw) && len(line) < 20 {
					if ptr, ok := sections[currentSection]; ok && len(sectionLines) > 0 {
						*ptr = strings.Join(sectionLines, "\n")
					}
					currentSection = section
					sectionLines = nil
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			sectionLines = append(sectionLines, line)
		}
	}

	if ptr, ok := sections[currentSection]; ok {
		*ptr = strings.Join(sectionLines, "\n")
	}

	return segments, nil
}

func (pg *ParseGraph) llmExtractNode(ctx context.Context, input *moduleSegments) (*domain.ParsedResume, error) {
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
				Desc:     "教育背景列表",
			},
			"work_experience": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "工作经历列表",
			},
			"projects": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object},
				Desc:     "项目经验列表",
			},
			"skills": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "技能列表",
			},
		}),
	}

	prompt := fmt.Sprintf(`请从以下简历文本中提取结构化信息。

个人信息部分：
%s

教育背景部分：
%s

工作经历部分：
%s

项目经验部分：
%s

技能部分：
%s

请调用 extract_resume_info 函数返回提取结果。`, input.PersonalInfo, input.Education, input.Work, input.Projects, input.Skills)

	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `你是一个专业的简历信息提取专家。你需要从简历文本中精准提取以下信息：
1. 个人信息：姓名、电话、邮箱、地址、个人简介
2. 教育背景：学校、学位、专业、起止时间
3. 工作经历：公司、职位、起止时间、工作描述
4. 项目经验：项目名、角色、起止时间、项目描述
5. 技能栈：所有技能关键词

你必须调用 extract_resume_info 函数来返回提取结果。确保提取的信息准确、完整。`,
		},
		{
			Role:    schema.User,
			Content: prompt,
		},
	}

	resp, err := pg.chatModel.Generate(ctx, messages, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		pg.l.Error("LLM调用失败", logger.Error(err))
		return pg.fallbackExtract(input), nil
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
				pg.l.Error("解析LLM返回结果失败", logger.Error(err))
				return pg.fallbackExtract(input), nil
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
		return pg.parseFromContent(resp.Content), nil
	}

	return pg.fallbackExtract(input), nil
}

func (pg *ParseGraph) schemaValidateNode(ctx context.Context, input *domain.ParsedResume) (*domain.ParsedResume, error) {
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

func (pg *ParseGraph) fallbackExtract(input *moduleSegments) *domain.ParsedResume {
	result := &domain.ParsedResume{
		PersonalInfo: domain.PersonalInfo{
			Name:    "未知",
			Phone:   "",
			Email:   "",
			Address: "",
			Summary: input.PersonalInfo,
		},
		Education:      []domain.Education{},
		WorkExperience: []domain.WorkExperience{},
		Projects:       []domain.Project{},
		Skills:         []string{},
	}
	return result
}

func (pg *ParseGraph) parseFromContent(content string) *domain.ParsedResume {
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
