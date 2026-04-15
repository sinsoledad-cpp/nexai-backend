package middleware

import (
	"errors"
	"net/http"
	jwtware "nexai-backend/internal/common/jwt"
	"strings"
	"time"

	"github.com/ecodeclub/ekit/set"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type JWTAuth struct {
	publicPaths set.Set[string]
	hdl         jwtware.Handler
}

func NewJWTAuth(hdl jwtware.Handler) *JWTAuth {
	s := set.NewMapSet[string](16)
	s.Add("/v1/users/signup")
	s.Add("/v1/users/login")
	s.Add("/v1/users/login/sms")
	s.Add("/v1/users/verification-codes/login")
	s.Add("/v1/users/verification-codes/reset-password")
	s.Add("/v1/users/password/reset")
	s.Add("/v1/users/token")
	return &JWTAuth{
		publicPaths: s,
		hdl:         hdl,
	}
}

func (j *JWTAuth) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if j.publicPaths.Exist(ctx.Request.URL.Path) {
			return
		}
		if strings.HasPrefix(ctx.Request.URL.Path, "/uploads/") ||
			strings.HasPrefix(ctx.Request.URL.Path, "/storage/") {
			return
		}
		tokenStr := j.hdl.ExtractTokenString(ctx)
		uc := jwtware.UserClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, &uc, func(token *jwt.Token) (interface{}, error) {
			return jwtware.AccessTokenKey, nil
		})
		if err != nil || !token.Valid {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		expireTime, err := uc.GetExpirationTime()
		if err != nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if expireTime.Before(time.Now()) {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		err = j.hdl.CheckSession(ctx, uc.Ssid)
		if err != nil {
			if errors.Is(err, jwtware.ErrSessionNotFound) {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			ctx.Set("user", uc)
			return
		}

		ctx.Set("user", uc)
	}
}
