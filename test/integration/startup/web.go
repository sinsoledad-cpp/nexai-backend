package startup

import (
	jwtware "nexai-backend/internal/common/jwt"
	"nexai-backend/internal/user/handler"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func projectRoot() string {
	return "../.."
}

func InitGinServer(hdl *handler.UserHandler, jwtHdl jwtware.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	server := gin.Default()
	server.Static("/uploads", filepath.Join(projectRoot(), "storage", "uploads"))
	// 暂时不使用 JWT 中间件，因为测试中会手动设置 token
	hdl.RegisterRoutes(server)
	return server
}
