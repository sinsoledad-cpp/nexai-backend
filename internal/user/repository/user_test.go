package repository

import (
	"context"
	"database/sql"
	"errors"
	"nexai-backend/internal/user/domain"
	"nexai-backend/internal/user/repository/cache"
	cachemocks "nexai-backend/internal/user/repository/cache/mocks"
	"nexai-backend/internal/user/repository/dao"
	daomocks "nexai-backend/internal/user/repository/dao/mocks"
	"nexai-backend/pkg/logger"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCachedUserRepository_FindById(t *testing.T) {
	t.Parallel()
	now := time.Now()
	now = time.UnixMilli(now.UnixMilli())

	testCases := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache)
		ctx      context.Context
		uid      int64
		wantErr  error
		wantUser domain.User
	}{
		{
			name: "cache hit",
			uid:  1,
			ctx:  context.Background(),
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				c.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{
					ID:       1,
					Email:    "test@example.com",
					Nickname: "test_user",
					Birthday: now,
					Phone:    "12345678901",
					Ctime:    now,
				}, nil)
				return d, c
			},
			wantUser: domain.User{
				ID:       1,
				Email:    "test@example.com",
				Nickname: "test_user",
				Birthday: now,
				Phone:    "12345678901",
				Ctime:    now,
			},
			wantErr: nil,
		},
		{
			name: "cache miss and dao hit",
			uid:  1,
			ctx:  context.Background(),
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				c.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{}, cache.ErrKeyNotExist)
				d.EXPECT().FindById(gomock.Any(), int64(1)).Return(dao.User{
					ID: 1,
					Email: sql.NullString{
						String: "test@example.com",
						Valid:  true,
					},
					Nickname: "test_user",
					Phone: sql.NullString{
						String: "12345678901",
						Valid:  true,
					},
					Birthday: sql.NullInt64{
						Int64: now.UnixMilli(),
						Valid: true,
					},
					Ctime: now.UnixMilli(),
				}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any()).Return(nil)
				return d, c
			},
			wantUser: domain.User{
				ID:       1,
				Email:    "test@example.com",
				Nickname: "test_user",
				Birthday: now,
				Phone:    "12345678901",
				Ctime:    now,
			},
			wantErr: nil,
		},
		{
			name: "cache miss and dao miss",
			uid:  1,
			ctx:  context.Background(),
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				c.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{}, cache.ErrKeyNotExist)
				d.EXPECT().FindById(gomock.Any(), int64(1)).Return(dao.User{}, errors.New("record not found"))
				return d, c
			},
			wantUser: domain.User{},
			wantErr:  errors.New("record not found"),
		},
		{
			name: "cache error",
			uid:  1,
			ctx:  context.Background(),
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				c.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{}, errors.New("redis error"))
				return d, c
			},
			wantUser: domain.User{},
			wantErr:  errors.New("redis error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := NewCachedUserRepository(d, c, logger.NewNopLogger())
			user, err := repo.FindById(tc.ctx, tc.uid)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)
		})
	}
}

func TestCachedUserRepository_Create(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache)
		ctx     context.Context
		user    domain.User
		wantErr error
	}{
		{
			name: "success",
			ctx:  context.Background(),
			user: domain.User{
				Email:    "test@example.com",
				Password: "password",
			},
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
				return d, c
			},
			wantErr: nil,
		},
		{
			name: "duplicate email",
			ctx:  context.Background(),
			user: domain.User{
				Email:    "test@example.com",
				Password: "password",
			},
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(dao.ErrDuplicateEmail)
				return d, c
			},
			wantErr: dao.ErrDuplicateEmail,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := NewCachedUserRepository(d, c, logger.NewNopLogger())
			err := repo.Create(tc.ctx, tc.user)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestCachedUserRepository_FindByEmail(t *testing.T) {
	t.Parallel()
	now := time.Now()
	now = time.UnixMilli(now.UnixMilli())

	testCases := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache)
		ctx      context.Context
		email    string
		wantErr  error
		wantUser domain.User
	}{
		{
			name:  "success",
			ctx:   context.Background(),
			email: "test@example.com",
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(dao.User{
					ID: 1,
					Email: sql.NullString{
						String: "test@example.com",
						Valid:  true,
					},
					Ctime: now.UnixMilli(),
				}, nil)
				return d, c
			},
			wantUser: domain.User{
				ID:    1,
				Email: "test@example.com",
				Ctime: now,
			},
			wantErr: nil,
		},
		{
			name:  "not found",
			ctx:   context.Background(),
			email: "test@example.com",
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(dao.User{}, dao.ErrRecordNotFound)
				return d, c
			},
			wantUser: domain.User{},
			wantErr:  dao.ErrRecordNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := NewCachedUserRepository(d, c, logger.NewNopLogger())
			user, err := repo.FindByEmail(tc.ctx, tc.email)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)
		})
	}
}

func TestCachedUserRepository_FindByPhone(t *testing.T) {
	t.Parallel()
	now := time.Now()
	now = time.UnixMilli(now.UnixMilli())

	testCases := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache)
		ctx      context.Context
		phone    string
		wantErr  error
		wantUser domain.User
	}{
		{
			name:  "success",
			ctx:   context.Background(),
			phone: "12345678901",
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(dao.User{
					ID: 1,
					Phone: sql.NullString{
						String: "12345678901",
						Valid:  true,
					},
					Ctime: now.UnixMilli(),
				}, nil)
				return d, c
			},
			wantUser: domain.User{
				ID:    1,
				Phone: "12345678901",
				Ctime: now,
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := NewCachedUserRepository(d, c, logger.NewNopLogger())
			user, err := repo.FindByPhone(tc.ctx, tc.phone)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)
		})
	}
}

func TestCachedUserRepository_UpdateAvatar(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache)
		ctx     context.Context
		id      int64
		avatar  string
		wantErr error
	}{
		{
			name:   "success",
			ctx:    context.Background(),
			id:     1,
			avatar: "avatar.jpg",
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().UpdateAvatar(gomock.Any(), int64(1), "avatar.jpg").Return(nil)
				c.EXPECT().Delete(gomock.Any(), int64(1)).Return(nil)
				return d, c
			},
			wantErr: nil,
		},
		{
			name:   "dao error",
			ctx:    context.Background(),
			id:     1,
			avatar: "avatar.jpg",
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().UpdateAvatar(gomock.Any(), int64(1), "avatar.jpg").Return(errors.New("db error"))
				return d, c
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := NewCachedUserRepository(d, c, logger.NewNopLogger())
			err := repo.UpdateAvatar(tc.ctx, tc.id, tc.avatar)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestCachedUserRepository_UpdateNonZeroFields(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache)
		ctx     context.Context
		user    domain.User
		wantErr error
	}{
		{
			name: "success",
			ctx:  context.Background(),
			user: domain.User{
				ID:       1,
				Nickname: "new nickname",
			},
			mock: func(ctrl *gomock.Controller) (dao.UserDAO, *cachemocks.MockUserCache) {
				d := daomocks.NewMockUserDAO(ctrl)
				c := cachemocks.NewMockUserCache(ctrl)
				d.EXPECT().UpdateById(gomock.Any(), gomock.Any()).Return(nil)
				c.EXPECT().Delete(gomock.Any(), int64(1)).Return(nil)
				// The async delayed delete is tricky to test deterministically in unit test without a time mock or sleep.
				// For this test we assume success if initial delete succeeds.
				return d, c
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d, c := tc.mock(ctrl)
			repo := NewCachedUserRepository(d, c, logger.NewNopLogger())
			err := repo.UpdateNonZeroFields(tc.ctx, tc.user)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
