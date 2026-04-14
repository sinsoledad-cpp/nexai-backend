//go:build wireinject

package main

import (
	ioc2 "nexai-backend/cmd/server/ioc"
	coderepo "nexai-backend/internal/code/repository"
	codecache "nexai-backend/internal/code/repository/cache"
	codeservice "nexai-backend/internal/code/service"
	"nexai-backend/internal/common/jwt"
	"nexai-backend/internal/user/handler"
	userrepo "nexai-backend/internal/user/repository"
	usercache "nexai-backend/internal/user/repository/cache"
	"nexai-backend/internal/user/repository/dao"
	userservice "nexai-backend/internal/user/service"

	"github.com/google/wire"
)

var thirdParty = wire.NewSet(
	ioc2.InitLogger,
	ioc2.InitPostgreSQL,
	ioc2.InitRedis,
)

var userSvc = wire.NewSet(
	usercache.NewRedisUserCache,
	dao.NewGORMUserDAO,
	userrepo.NewCachedUserRepository,
	userservice.NewUserService,
)

var codeSvc = wire.NewSet(
	codecache.NewRedisCodeCache,
	coderepo.NewCachedCodeRepository,
	ioc2.InitSMSService,
	codeservice.NewCodeService,
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
