package repository

import (
	"context"
	"errors"
	cachemocks "nexai-backend/internal/code/repository/cache/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCachedCodeRepository_Set(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) *cachemocks.MockCodeCache
		ctx     context.Context
		biz     string
		phone   string
		code    string
		wantErr error
	}{
		{
			name:  "success",
			ctx:   context.Background(),
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(ctrl *gomock.Controller) *cachemocks.MockCodeCache {
				c := cachemocks.NewMockCodeCache(ctrl)
				c.EXPECT().Set(gomock.Any(), "login", "12345678901", "123456").Return(nil)
				return c
			},
			wantErr: nil,
		},
		{
			name:  "cache error",
			ctx:   context.Background(),
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(ctrl *gomock.Controller) *cachemocks.MockCodeCache {
				c := cachemocks.NewMockCodeCache(ctrl)
				c.EXPECT().Set(gomock.Any(), "login", "12345678901", "123456").Return(errors.New("redis error"))
				return c
			},
			wantErr: errors.New("redis error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := tc.mock(ctrl)
			repo := NewCachedCodeRepository(c)
			err := repo.Set(tc.ctx, tc.biz, tc.phone, tc.code)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestCachedCodeRepository_Verify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) *cachemocks.MockCodeCache
		ctx     context.Context
		biz     string
		phone   string
		code    string
		wantOk  bool
		wantErr error
	}{
		{
			name:   "success",
			ctx:    context.Background(),
			biz:    "login",
			phone:  "12345678901",
			code:   "123456",
			wantOk: true,
			mock: func(ctrl *gomock.Controller) *cachemocks.MockCodeCache {
				c := cachemocks.NewMockCodeCache(ctrl)
				c.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(true, nil)
				return c
			},
			wantErr: nil,
		},
		{
			name:   "verify failed",
			ctx:    context.Background(),
			biz:    "login",
			phone:  "12345678901",
			code:   "123456",
			wantOk: false,
			mock: func(ctrl *gomock.Controller) *cachemocks.MockCodeCache {
				c := cachemocks.NewMockCodeCache(ctrl)
				c.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, nil)
				return c
			},
			wantErr: nil,
		},
		{
			name:   "cache error",
			ctx:    context.Background(),
			biz:    "login",
			phone:  "12345678901",
			code:   "123456",
			wantOk: false,
			mock: func(ctrl *gomock.Controller) *cachemocks.MockCodeCache {
				c := cachemocks.NewMockCodeCache(ctrl)
				c.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, errors.New("redis error"))
				return c
			},
			wantErr: errors.New("redis error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := tc.mock(ctrl)
			repo := NewCachedCodeRepository(c)
			ok, err := repo.Verify(tc.ctx, tc.biz, tc.phone, tc.code)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}
