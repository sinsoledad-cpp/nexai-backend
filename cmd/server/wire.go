//go:build wireinject

package main

import (
	ioc2 "nexai-backend/cmd/server/ioc"
	coderepo "nexai-backend/internal/code/repository"
	codecache "nexai-backend/internal/code/repository/cache"
	"nexai-backend/internal/code/service"
	"nexai-backend/internal/common/jwt"
	"nexai-backend/internal/user/handler"
	userrepo "nexai-backend/internal/user/repository"
	usercache "nexai-backend/internal/user/repository/cache"
	"nexai-backend/internal/user/repository/dao"
	"nexai-backend/internal/user/service"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

type App struct {
	engine *gin.Engine
}

var thirdParty = wire.NewSet(
	ioc2.InitLogger,
	ioc2.InitMySQL,
	ioc2.InitRedis,
)

var userSvc = wire.NewSet(
	usercache.NewRedisUserCache,
	dao.NewGORMUserDAO,
	userrepo.NewCachedUserRepository,
	service.NewUserService,
)

var codeSvc = wire.NewSet(
	codecache.NewRedisCodeCache,
	coderepo.NewCachedCodeRepository,
	ioc2.InitSMSService,
	service.NewCodeService,
)

func InitApp() *App {
	wire.Build(
		thirdParty,

		userSvc,
		codeSvc,

		jwt.NewRedisJWTHandler,
		handler.NewUserHandler,

		ioc2.InitWebEngine,
		ioc2.InitGinMiddlewares,
		wire.Struct(new(App), "*"),
	)
	return new(App)
}
