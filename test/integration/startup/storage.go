package startup

// import (
// 	"nexai-backend/pkg/storage"
// 	"nexai-backend/pkg/storage/local"
// 	"path/filepath"
// 	"runtime"
// )

// func InitStorageService() storage.Provider {
// 	return local.NewProvider(local.Config{
// 		RootPath: filepath.Join(projectRoot(), "storage", "uploads"),
// 		BaseURL:  "http://localhost:8080/uploads",
// 	})
// }

// func projectRoot() string {
// 	_, f, _, ok := runtime.Caller(0)
// 	if !ok {
// 		return "."
// 	}
// 	return filepath.Clean(filepath.Join(filepath.Dir(f), "..", "..", ".."))
// }
