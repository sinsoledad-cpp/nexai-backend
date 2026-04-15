package domain

import "time"

// Resume 简历实体
type Resume struct {
	ID       int64        // 简历ID
	UserID   int64        // 用户ID
	FileName string       // 文件名
	FileURL  string       // 文件存储路径
	FileType string       // 文件类型（.pdf, .docx, .jpg等）
	RawText  string       // 提取的原始文本
	Parsed   ParsedResume // 解析后的结构化数据
	Status   ResumeStatus // 简历状态
	Ctime    time.Time    // 创建时间
	Utime    time.Time    // 更新时间
}

// ResumeStatus 简历状态枚举
type ResumeStatus int

const (
	StatusUploaded    ResumeStatus = iota // 已上传
	StatusParsing                         // 解析中
	StatusParsed                          // 解析完成
	StatusParseFailed                     // 解析失败
	StatusScored                          // 已评分
)

// ParsedResume 解析后的结构化简历数据
type ParsedResume struct {
	PersonalInfo   PersonalInfo     `json:"personalInfo"`   // 个人信息
	Education      []Education      `json:"education"`      // 教育背景
	WorkExperience []WorkExperience `json:"workExperience"` // 工作经历
	Projects       []Project        `json:"projects"`       // 项目经验
	Skills         []string         `json:"skills"`         // 技能栈
}

// PersonalInfo 个人信息
type PersonalInfo struct {
	Name    string `json:"name"`    // 姓名
	Phone   string `json:"phone"`   // 手机号码
	Email   string `json:"email"`   // 邮箱地址
	Address string `json:"address"` // 地址
	Summary string `json:"summary"` // 个人简介/优势
}

// Education 教育背景
type Education struct {
	School    string `json:"school"`    // 学校名称
	Degree    string `json:"degree"`    // 学位
	Major     string `json:"major"`     // 专业
	StartDate string `json:"startDate"` // 开始时间
	EndDate   string `json:"endDate"`   // 结束时间
}

// WorkExperience 工作经历
type WorkExperience struct {
	Company     string `json:"company"`     // 公司名称
	Position    string `json:"position"`    // 职位
	StartDate   string `json:"startDate"`   // 开始时间
	EndDate     string `json:"endDate"`     // 结束时间
	Description string `json:"description"` // 工作描述
}

// Project 项目经验
type Project struct {
	Name        string `json:"name"`        // 项目名称
	Role        string `json:"role"`        // 角色
	StartDate   string `json:"startDate"`   // 开始时间
	EndDate     string `json:"endDate"`     // 结束时间
	Description string `json:"description"` // 项目描述
}
