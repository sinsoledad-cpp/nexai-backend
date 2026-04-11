package ioc

// import (
// 	"nexai-backend/pkg/storage"
// 	"nexai-backend/pkg/storage/local"
// )

// func InitStorageService() storage.Provider {
// 	// 这里可以根据配置决定使用本地存储还是 OSS 等
// 	// 目前先硬编码为本地存储
// 	// 实际项目中应该从配置文件读取
// 	c := local.Config{
// 		RootPath: "./storage/uploads",
// 		BaseURL:  "http://localhost:8080/uploads",
// 	}
// 	return local.NewProvider(c)
// }
