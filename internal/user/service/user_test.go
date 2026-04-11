package service

import (
	"context"
	"errors"
	"testing"

	"nexai-backend/internal/user/domain"
	"nexai-backend/internal/user/repository"
	"nexai-backend/internal/user/repository/mocks"
	"nexai-backend/pkg/logger"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestUserService_Signup(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) repository.UserRepository
		user    domain.User
		wantErr error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, u domain.User) error {
					// 验证密码是否加密
					assert.NotEqual(t, "password", u.Password)
					// 验证加密后的密码是否正确
					err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("password"))
					assert.NoError(t, err)
					return nil
				})
				return repo
			},
			user: domain.User{
				Email:    "test@example.com",
				Password: "password",
			},
			wantErr: nil,
		},
		{
			name: "duplicate email",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrDuplicateEmail)
				return repo
			},
			user: domain.User{
				Email:    "test@example.com",
				Password: "password",
			},
			wantErr: repository.ErrDuplicateEmail,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewUserService(logger.NewNopLogger(), repo)
			err := svc.Signup(context.Background(), tc.user)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestUserService_Login(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) repository.UserRepository
		email    string
		password string
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				// 生成一个加密后的密码
				hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(domain.User{
					ID:       1,
					Email:    "test@example.com",
					Password: string(hash),
				}, nil)
				return repo
			},
			email:    "test@example.com",
			password: "password",
			wantUser: domain.User{
				ID:    1,
				Email: "test@example.com",
				// Password check is tricky because we return the user from repo
			},
			wantErr: nil,
		},
		{
			name: "user not found",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(domain.User{}, repository.ErrUserNotFound)
				return repo
			},
			email:    "test@example.com",
			password: "password",
			wantUser: domain.User{},
			wantErr:  ErrInvalidUserOrPassword,
		},
		{
			name: "password invalid",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				// 生成一个加密后的密码
				hash, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(domain.User{
					ID:       1,
					Email:    "test@example.com",
					Password: string(hash),
				}, nil)
				return repo
			},
			email:    "test@example.com",
			password: "wrong_password",
			wantUser: domain.User{},
			wantErr:  ErrInvalidUserOrPassword,
		},
		{
			name: "repository error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(domain.User{}, errors.New("db error"))
				return repo
			},
			email:    "test@example.com",
			password: "password",
			wantUser: domain.User{},
			wantErr:  errors.New("db error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewUserService(logger.NewNopLogger(), repo)
			user, err := svc.Login(context.Background(), tc.email, tc.password)
			assert.Equal(t, tc.wantErr, err)
			if err == nil {
				assert.Equal(t, tc.wantUser.ID, user.ID)
				assert.Equal(t, tc.wantUser.Email, user.Email)
			}
		})
	}
}

func TestUserService_FindOrCreate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) repository.UserRepository
		phone    string
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "found existing user",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{
					ID:    1,
					Phone: "12345678901",
				}, nil)
				return repo
			},
			phone: "12345678901",
			wantUser: domain.User{
				ID:    1,
				Phone: "12345678901",
			},
			wantErr: nil,
		},
		{
			name: "create new user",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				// 第一次找没找到
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{}, repository.ErrUserNotFound)
				// 创建
				repo.EXPECT().Create(gomock.Any(), domain.User{Phone: "12345678901"}).Return(nil)
				// 再次查找
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{
					ID:    1,
					Phone: "12345678901",
				}, nil)
				return repo
			},
			phone: "12345678901",
			wantUser: domain.User{
				ID:    1,
				Phone: "12345678901",
			},
			wantErr: nil,
		},
		{
			name: "create conflict",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				// 第一次找没找到
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{}, repository.ErrUserNotFound)
				// 创建时并发冲突
				repo.EXPECT().Create(gomock.Any(), domain.User{Phone: "12345678901"}).Return(repository.ErrDuplicatePhone)
				// 再次查找
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{
					ID:    1,
					Phone: "12345678901",
				}, nil)
				return repo
			},
			phone: "12345678901",
			wantUser: domain.User{
				ID:    1,
				Phone: "12345678901",
			},
			wantErr: nil,
		},
		{
			name: "find error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{}, errors.New("db error"))
				return repo
			},
			phone:    "12345678901",
			wantUser: domain.User{},
			wantErr:  errors.New("db error"),
		},
		{
			name: "create error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindByPhone(gomock.Any(), "12345678901").Return(domain.User{}, repository.ErrUserNotFound)
				repo.EXPECT().Create(gomock.Any(), domain.User{Phone: "12345678901"}).Return(errors.New("create error"))
				return repo
			},
			phone:    "12345678901",
			wantUser: domain.User{},
			wantErr:  errors.New("create error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewUserService(logger.NewNopLogger(), repo)
			user, err := svc.FindOrCreate(context.Background(), tc.phone)
			assert.Equal(t, tc.wantErr, err)
			if err == nil {
				assert.Equal(t, tc.wantUser.ID, user.ID)
			}
		})
	}
}

func TestUserService_UpdateAvatarPath(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) repository.UserRepository
		uid     int64
		newPath string
		wantErr error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindById(gomock.Any(), int64(1)).Return(domain.User{
					ID:     1,
					Avatar: "old/path/avatar.jpg",
				}, nil)
				repo.EXPECT().UpdateAvatar(gomock.Any(), int64(1), "new/path/avatar.jpg").Return(nil)
				return repo
			},
			uid:     1,
			newPath: "new/path/avatar.jpg",
			wantErr: nil,
		},
		{
			name: "find user error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindById(gomock.Any(), int64(1)).Return(domain.User{}, errors.New("db error"))
				return repo
			},
			uid:     1,
			newPath: "new/path/avatar.jpg",
			wantErr: errors.New("db error"),
		},
		{
			name: "update avatar error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindById(gomock.Any(), int64(1)).Return(domain.User{
					ID:     1,
					Avatar: "old/path/avatar.jpg",
				}, nil)
				repo.EXPECT().UpdateAvatar(gomock.Any(), int64(1), "new/path/avatar.jpg").Return(errors.New("update error"))
				return repo
			},
			uid:     1,
			newPath: "new/path/avatar.jpg",
			wantErr: errors.New("update error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewUserService(logger.NewNopLogger(), repo)
			err := svc.UpdateAvatarPath(context.Background(), tc.uid, tc.newPath)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestUserService_FindById(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) repository.UserRepository
		uid      int64
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindById(gomock.Any(), int64(1)).Return(domain.User{
					ID: 1,
				}, nil)
				return repo
			},
			uid: 1,
			wantUser: domain.User{
				ID: 1,
			},
			wantErr: nil,
		},
		{
			name: "error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().FindById(gomock.Any(), int64(1)).Return(domain.User{}, errors.New("db error"))
				return repo
			},
			uid:      1,
			wantUser: domain.User{},
			wantErr:  errors.New("db error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewUserService(logger.NewNopLogger(), repo)
			user, err := svc.FindById(context.Background(), tc.uid)
			assert.Equal(t, tc.wantErr, err)
			if err == nil {
				assert.Equal(t, tc.wantUser.ID, user.ID)
			}
		})
	}
}

func TestUserService_UpdateNonSensitiveInfo(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) repository.UserRepository
		user    domain.User
		wantErr error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().UpdateNonZeroFields(gomock.Any(), domain.User{ID: 1}).Return(nil)
				return repo
			},
			user:    domain.User{ID: 1},
			wantErr: nil,
		},
		{
			name: "error",
			mock: func(ctrl *gomock.Controller) repository.UserRepository {
				repo := mocks.NewMockUserRepository(ctrl)
				repo.EXPECT().UpdateNonZeroFields(gomock.Any(), domain.User{ID: 1}).Return(errors.New("db error"))
				return repo
			},
			user:    domain.User{ID: 1},
			wantErr: errors.New("db error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			svc := NewUserService(logger.NewNopLogger(), repo)
			err := svc.UpdateNonSensitiveInfo(context.Background(), tc.user)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
