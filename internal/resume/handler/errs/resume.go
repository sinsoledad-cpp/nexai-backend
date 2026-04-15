package errs

// 错误码定义
const (
	ResumeInvalidInput        = 402001 // 无效输入
	ResumeInternalServerError = 502001 // 服务器内部错误
	ResumeFileNotFound        = 402002 // 文件不存在
	ResumeFileTypeUnsupported = 402003 // 文件类型不支持
	ResumeFileTooLarge        = 402004 // 文件过大
	ResumeParseFailed         = 502002 // 解析失败
	ResumeScoreFailed         = 502003 // 评分失败
	ResumeNotParsed           = 402005 // 未解析
)
