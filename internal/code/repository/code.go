package repository

import (
	"context"
	"nexai-backend/internal/code/repository/cache"
)

var ErrCodeVerifyTooMany = cache.ErrCodeVerifyTooMany
var ErrCodeSendTooMany = cache.ErrCodeSendTooMany
var ErrCodeExpired = cache.ErrCodeExpired

//go:generate mockgen -source=./code.go -package=mocks -destination=./mocks/code_mock.go CodeRepository
type CodeRepository interface {
	Set(ctx context.Context, biz, phone, code string) error
	Verify(ctx context.Context, biz, phone, code string) (bool, error)
}

type CachedCodeRepository struct {
	cache cache.CodeCache
}

func NewCachedCodeRepository(c cache.CodeCache) CodeRepository {
	return &CachedCodeRepository{
		cache: c,
	}
}

func (c *CachedCodeRepository) Set(ctx context.Context, biz, phone, code string) error {
	return c.cache.Set(ctx, biz, phone, code)
}

func (c *CachedCodeRepository) Verify(ctx context.Context, biz, phone, code string) (bool, error) {
	return c.cache.Verify(ctx, biz, phone, code)
}
