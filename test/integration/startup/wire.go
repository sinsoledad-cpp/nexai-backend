//go:build wireinject

package startup

import (
	codeRepo "nexai-backend/internal/code/repository"
	"nexai-backend/internal/common/jwt"
	"nexai-backend/internal/common/sms/memory"
	"nexai-backend/internal/user/handler"
	userRepo "nexai-backend/internal/user/repository"
	userCache "nexai-backend/internal/user/repository/cache"
	userDAO "nexai-backend/internal/user/repository/dao"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

var thirdParty = wire.NewSet(
	InitLogger,
	InitMySQL,
	InitRedis,
	InitStorageService,
)

var userSvc = wire.NewSet(
	userCache.NewRedisUserCache,
	userDAO.NewGORMUserDAO,
	userRepo.NewCachedUserRepository,
	userSvc.NewUserService,
)

var codeSvc = wire.NewSet(
	codeCache.NewRedisCodeCache,
	codeRepo.NewCachedCodeRepository,
	codeSvc.NewCodeService,
	memory.NewService,
)

func InitUserHandler() *handler.UserHandler {
	wire.Build(
		thirdParty,
		userSvc,
		codeSvc,
		jwt.NewRedisJWTHandler,
		handler.NewUserHandler,
	)
	return new(handler.UserHandler)
}

func InitWebServer() *gin.Engine {
	wire.Build(
		thirdParty,
		userSvc,
		codeSvc,
		jwt.NewRedisJWTHandler,
		handler.NewUserHandler,
		InitGinServer,
	)
	return new(gin.Engine)
}
