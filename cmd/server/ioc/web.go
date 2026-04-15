package ioc

import (
	"context"
	"nexai-backend/internal/common/jwt"
	"nexai-backend/internal/common/middleware"
	resumehandler "nexai-backend/internal/resume/handler"
	"nexai-backend/internal/user/handler"
	"nexai-backend/pkg/ginx"
	ginxmw "nexai-backend/pkg/ginx/middleware"
	"nexai-backend/pkg/logger"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func InitWebEngine(middlewares []gin.HandlerFunc, l logger.Logger, userHdl *handler.UserHandler, resumeHdl *resumehandler.ResumeHandler) *gin.Engine {
	ginx.SetLogger(l)
	gin.ForceConsoleColor()
	engine := gin.Default()

	dirs := []string{"./storage/uploads/avatars", "./storage/avatar", "./storage/resumes"}
	for _, dir := range dirs {
		_ = os.MkdirAll(dir, 0755)
	}

	engine.Static("/storage", "./storage")
	engine.Static("/uploads", "./storage/uploads")
	engine.Use(middlewares...)
	userHdl.RegisterRoutes(engine)
	resumeHdl.RegisterRoutes(engine)
	return engine
}

func InitGinMiddlewares(jwtHdl jwt.Handler, l logger.Logger) []gin.HandlerFunc {
	corsMiddleware := cors.New(cors.Config{
		// 在生产环境中，您应该将 AllowAllOrigins 设置为 false，并具体指定允许的前端域名
		// 例如: AllowOrigins: []string{"http://your-frontend.com"},
		// AllowAllOrigins: true,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Refresh-Token"},
		// 允许前端访问后端设置的响应头
		ExposeHeaders: []string{"X-Jwt-Token", "X-Refresh-Token"},
		// 允许携带 Cookie
		AllowCredentials: true,
		// preflight 请求的缓存时间
		MaxAge: 12 * time.Hour,
	})
	logFn := func(ctx context.Context, al ginxmw.AccessLog) {
		fields := []logger.Field{
			logger.String("path", al.Path),
			logger.String("method", al.Method),
			logger.String("req_body", al.ReqBody),
			logger.Int("status", al.Status),
			logger.String("resp_body", al.RespBody),
			logger.Int64("duration_ms", al.Duration.Milliseconds()),
		}
		l.Info("access log ", fields...)
	}
	accessLogMiddleware := ginxmw.NewAccessLogBuilder(logFn).AllowReqBody().AllowRespBody().Build()
	return []gin.HandlerFunc{
		otelgin.Middleware("bedrock"),
		corsMiddleware,
		middleware.NewJWTAuth(jwtHdl).Middleware(),
		accessLogMiddleware,
	}
}
