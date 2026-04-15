package jwt

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var _ Handler = &RedisJWTHandler{}

type RedisJWTHandler struct {
	client        redis.Cmdable
	signingMethod jwt.SigningMethod
	rcExpiration  time.Duration
}

func NewRedisJWTHandler(client redis.Cmdable) Handler {
	return &RedisJWTHandler{
		client:        client,
		signingMethod: jwt.SigningMethodHS512,
		rcExpiration:  time.Hour * 24 * 7,
	}
}

func (r *RedisJWTHandler) CheckSession(ctx *gin.Context, ssid string) error {
	cnt, err := r.client.Exists(ctx, fmt.Sprintf("users:ssid:%s", ssid)).Result()
	if err != nil {
		return err
	}
	if cnt > 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *RedisJWTHandler) ExtractTokenString(ctx *gin.Context) string {
	authCode := ctx.GetHeader("Authorization")
	if authCode == "" {
		return authCode
	}
	segs := strings.Split(authCode, " ")
	if len(segs) != 2 {
		return ""
	}
	return segs[1]
}

func (r *RedisJWTHandler) SetLoginToken(ctx *gin.Context, uid int64) (TokenPair, error) {
	ssid := uuid.New().String()
	refreshToken, err := r.generateRefreshToken(uid, ssid)
	if err != nil {
		return TokenPair{}, err
	}
	accessToken, err := r.generateAccessToken(uid, ssid)
	if err != nil {
		return TokenPair{}, err
	}
	ctx.Header("X-Jwt-Token", accessToken)
	ctx.Header("X-Refresh-Token", refreshToken)
	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (r *RedisJWTHandler) ClearToken(ctx *gin.Context) error {
	ctx.Header("X-Jwt-Token", "")
	ctx.Header("X-Refresh-Token", "")
	val, exists := ctx.Get("user")
	if !exists {
		return nil
	}
	uc, ok := val.(UserClaims)
	if !ok {
		return nil
	}
	return r.client.Set(ctx, fmt.Sprintf("users:ssid:%s", uc.Ssid), "", r.rcExpiration).Err()
}

func (r *RedisJWTHandler) SetJWTToken(ctx *gin.Context, uid int64, ssid string) (string, error) {
	tokenStr, err := r.generateAccessToken(uid, ssid)
	if err != nil {
		return "", err
	}
	ctx.Header("X-Jwt-Token", tokenStr)
	return tokenStr, nil
}

func (r *RedisJWTHandler) generateAccessToken(uid int64, ssid string) (string, error) {
	uc := UserClaims{
		Uid:  uid,
		Ssid: ssid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 60)),
		},
	}
	token := jwt.NewWithClaims(r.signingMethod, uc)
	return token.SignedString(AccessTokenKey)
}

func (r *RedisJWTHandler) generateRefreshToken(uid int64, ssid string) (string, error) {
	rc := RefreshClaims{
		Uid:  uid,
		Ssid: ssid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(r.rcExpiration)),
		},
	}
	token := jwt.NewWithClaims(r.signingMethod, rc)
	return token.SignedString(RefreshTokenKey)
}
