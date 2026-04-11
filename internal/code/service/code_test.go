package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"nexai-backend/internal/code/repository"
	repoMocks "nexai-backend/internal/code/repository/mocks"
	smsMocks "nexai-backend/internal/common/sms/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCodeService_Send(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) (repository.CodeRepository, *smsMocks.MockService)
		biz     string
		phone   string
		wantErr error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) (repository.CodeRepository, *smsMocks.MockService) {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				smsSvc := smsMocks.NewMockService(ctrl)

				repo.EXPECT().Set(gomock.Any(), "login", "12345678901", gomock.Any()).DoAndReturn(func(ctx context.Context, biz, phone, code string) error {
					assert.Len(t, code, 6)
					smsSvc.EXPECT().Send(gomock.Any(), "1877556", []string{code}, "12345678901").Return(nil)
					return nil
				})
				return repo, smsSvc
			},
			biz:     "login",
			phone:   "12345678901",
			wantErr: nil,
		},
		{
			name: "repo set error",
			mock: func(ctrl *gomock.Controller) (repository.CodeRepository, *smsMocks.MockService) {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				smsSvc := smsMocks.NewMockService(ctrl)

				repo.EXPECT().Set(gomock.Any(), "login", "12345678901", gomock.Any()).Return(errors.New("redis error"))
				return repo, smsSvc
			},
			biz:     "login",
			phone:   "12345678901",
			wantErr: errors.New("redis error"),
		},
		{
			name: "sms send error",
			mock: func(ctrl *gomock.Controller) (repository.CodeRepository, *smsMocks.MockService) {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				smsSvc := smsMocks.NewMockService(ctrl)

				repo.EXPECT().Set(gomock.Any(), "login", "12345678901", gomock.Any()).DoAndReturn(func(ctx context.Context, biz, phone, code string) error {
					smsSvc.EXPECT().Send(gomock.Any(), "1877556", []string{code}, "12345678901").Return(errors.New("sms error"))
					return nil
				})
				return repo, smsSvc
			},
			biz:     "login",
			phone:   "12345678901",
			wantErr: errors.New("sms error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo, smsSvc := tc.mock(ctrl)
			svc := NewCodeService(repo, smsSvc)
			err := svc.Send(context.Background(), tc.biz, tc.phone)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestCodeService_Verify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		mock      func(ctrl *gomock.Controller) repository.CodeRepository
		biz       string
		phone     string
		inputCode string
		wantOk    bool
		wantErr   error
	}{
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) repository.CodeRepository {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				repo.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(true, nil)
				return repo
			},
			biz:       "login",
			phone:     "12345678901",
			inputCode: "123456",
			wantOk:    true,
			wantErr:   nil,
		},
		{
			name: "verify failed",
			mock: func(ctrl *gomock.Controller) repository.CodeRepository {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				repo.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, nil)
				return repo
			},
			biz:       "login",
			phone:     "12345678901",
			inputCode: "123456",
			wantOk:    false,
			wantErr:   nil,
		},
		{
			name: "verify too many",
			mock: func(ctrl *gomock.Controller) repository.CodeRepository {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				repo.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, repository.ErrCodeVerifyTooMany)
				return repo
			},
			biz:       "login",
			phone:     "12345678901",
			inputCode: "123456",
			wantOk:    false,
			wantErr:   repository.ErrCodeVerifyTooMany,
		},
		{
			name: "code expired",
			mock: func(ctrl *gomock.Controller) repository.CodeRepository {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				repo.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, repository.ErrCodeExpired)
				return repo
			},
			biz:       "login",
			phone:     "12345678901",
			inputCode: "123456",
			wantOk:    false,
			wantErr:   repository.ErrCodeExpired,
		},
		{
			name: "unknown error",
			mock: func(ctrl *gomock.Controller) repository.CodeRepository {
				repo := repoMocks.NewMockCodeRepository(ctrl)
				repo.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, fmt.Errorf("unknown error"))
				return repo
			},
			biz:       "login",
			phone:     "12345678901",
			inputCode: "123456",
			wantOk:    false,
			wantErr:   fmt.Errorf("unknown error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tc.mock(ctrl)
			// Mock SMS service is not needed for Verify
			smsSvc := smsMocks.NewMockService(ctrl)
			svc := NewCodeService(repo, smsSvc)
			ok, err := svc.Verify(context.Background(), tc.biz, tc.phone, tc.inputCode)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}
