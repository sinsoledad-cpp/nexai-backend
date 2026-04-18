package handler

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	codeservice "nexai-backend/internal/code/service"
	codemocks "nexai-backend/internal/code/service/mocks"
	jwtware "nexai-backend/internal/common/jwt"
	jwtmocks "nexai-backend/internal/common/jwt/mocks"
	"nexai-backend/internal/user/domain"
	"nexai-backend/internal/user/handler/dto"
	"nexai-backend/internal/user/handler/errs"
	"nexai-backend/internal/user/service"
	svcmocks "nexai-backend/internal/user/service/mocks"
	"nexai-backend/pkg/ginx"
	"nexai-backend/pkg/logger"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestUserHandler_SignUp(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) service.UserService
		req  dto.SignUpRequest

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "注册成功",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Signup(gomock.Any(), domain.User{
					Email:    "test@example.com",
					Password: "Password123!",
				}).Return(nil)
				return svc
			},
			req: dto.SignUpRequest{
				Email:           "test@example.com",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantResult: ginx.Result{
				Code: http.StatusCreated,
				Msg:  "注册成功",
			},
			wantErr: nil,
		},
		{
			name: "两次输入密码不一致",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				return svc
			},
			req: dto.SignUpRequest{
				Email:           "test@example.com",
				Password:        "Password123!",
				ConfirmPassword: "Password1234!",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "两次输入密码不同",
			},
			wantErr: nil,
		},
		{
			name: "邮箱格式错误",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				return svc
			},
			req: dto.SignUpRequest{
				Email:           "invalid-email",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "邮箱格式错误",
			},
			wantErr: nil,
		},
		{
			name: "密码格式错误",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				return svc
			},
			req: dto.SignUpRequest{
				Email:           "test@example.com",
				Password:        "123",
				ConfirmPassword: "123",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "密码长度至少为6位",
			},
			wantErr: nil,
		},
		{
			name: "邮箱冲突",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Signup(gomock.Any(), domain.User{
					Email:    "test@example.com",
					Password: "Password123!",
				}).Return(service.ErrDuplicateEmail)
				return svc
			},
			req: dto.SignUpRequest{
				Email:           "test@example.com",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantResult: ginx.Result{
				Code: errs.UserDuplicateEmail,
				Msg:  "邮箱冲突",
			},
			wantErr: service.ErrDuplicateEmail,
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().Signup(gomock.Any(), domain.User{
					Email:    "test@example.com",
					Password: "Password123!",
				}).Return(errors.New("service error"))
				return svc
			},
			req: dto.SignUpRequest{
				Email:           "test@example.com",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantResult: ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统错误",
			},
			wantErr: errors.New("service error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			// 使用 NewUserHandler 初始化，确保正则表达式等字段被正确初始化
			h := NewUserHandler(logger.NewNopLogger(), svc, nil, nil)

			// 构造 gin.Context
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			// 必须设置 Request，因为 h.SignUp 用到了 ctx.Request.Context()
			ctx.Request = httptest.NewRequest("POST", "/users/signup", nil)

			// 调用被测方法
			res, err := h.SignUp(ctx, tc.req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_LogoutJWT(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		mock       func(ctrl *gomock.Controller) jwtware.Handler
		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "退出成功",
			mock: func(ctrl *gomock.Controller) jwtware.Handler {
				hdl := jwtmocks.NewMockHandler(ctrl)
				hdl.EXPECT().ClearToken(gomock.Any()).Return(nil)
				return hdl
			},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "退出登录成功",
			},
			wantErr: nil,
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) jwtware.Handler {
				hdl := jwtmocks.NewMockHandler(ctrl)
				hdl.EXPECT().ClearToken(gomock.Any()).Return(errors.New("redis error"))
				return hdl
			},
			wantResult: ginx.Result{
				Code: http.StatusInternalServerError,
				Msg:  "系统错误",
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

			jwtHdl := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), nil, nil, jwtHdl)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/logout", nil)

			res, err := h.LogoutJWT(ctx)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_RefreshToken(t *testing.T) {
	t.Parallel()
	// Helper to generate token
	genToken := func(uid int64, ssid string) string {
		rc := jwtware.RefreshClaims{
			Uid:  uid,
			Ssid: ssid,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, rc)
		s, _ := token.SignedString(jwtware.RefreshTokenKey)
		return s
	}

	testCases := []struct {
		name       string
		mock       func(ctrl *gomock.Controller) jwtware.Handler
		token      string
		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "刷新成功",
			mock: func(ctrl *gomock.Controller) jwtware.Handler {
				hdl := jwtmocks.NewMockHandler(ctrl)
				hdl.EXPECT().CheckSession(gomock.Any(), "ssid-123").Return(nil)
				hdl.EXPECT().SetJWTToken(gomock.Any(), int64(123), "ssid-123").Return("access_token", nil)
				return hdl
			},
			token: genToken(123, "ssid-123"),
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "刷新成功",
				Data: dto.RefreshTokenResponse{AccessToken: "access_token"},
			},
			wantErr: nil,
		},
		{
			name: "会话失效",
			mock: func(ctrl *gomock.Controller) jwtware.Handler {
				hdl := jwtmocks.NewMockHandler(ctrl)
				hdl.EXPECT().CheckSession(gomock.Any(), "ssid-123").Return(jwtware.ErrSessionNotFound)
				return hdl
			},
			token: genToken(123, "ssid-123"),
			wantResult: ginx.Result{
				Code: http.StatusUnauthorized,
				Msg:  "会话已过期，请重新登录",
			},
			wantErr: jwtware.ErrSessionNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			jwtHdl := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), nil, nil, jwtHdl)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/refresh_token", nil)

			req := dto.RefreshTokenRequest{RefreshToken: tc.token}
			res, err := h.RefreshToken(ctx, req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_LoginJWT(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler)
		req  dto.LoginRequest

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "登录成功",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				jwtHdl := jwtmocks.NewMockHandler(ctrl)
				svc.EXPECT().Login(gomock.Any(), "test@example.com", "Password123!").Return(domain.User{
					ID: 123,
				}, nil)
				jwtHdl.EXPECT().SetLoginToken(gomock.Any(), int64(123)).Return(jwtware.TokenPair{
					AccessToken:  "access_token",
					RefreshToken: "refresh_token",
				}, nil)
				return svc, jwtHdl
			},
			req: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "Password123!",
			},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "登录成功",
				Data: dto.LoginResponse{
					AccessToken:  "access_token",
					RefreshToken: "refresh_token",
					User:         dto.ProfileResponse{ID: 123},
				},
			},
			wantErr: nil,
		},
		{
			name: "用户名或者密码错误",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				jwtHdl := jwtmocks.NewMockHandler(ctrl)
				svc.EXPECT().Login(gomock.Any(), "test@example.com", "Password123!").Return(domain.User{}, service.ErrInvalidUserOrPassword)
				return svc, jwtHdl
			},
			req: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "Password123!",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidOrPassword,
				Msg:  "用户名或者密码错误",
			},
			wantErr: service.ErrInvalidUserOrPassword,
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				jwtHdl := jwtmocks.NewMockHandler(ctrl)
				svc.EXPECT().Login(gomock.Any(), "test@example.com", "Password123!").Return(domain.User{
					ID: 123,
				}, errors.New("service error"))
				return svc, jwtHdl
			},
			req: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "Password123!",
			},
			wantResult: ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统错误",
			},
			wantErr: errors.New("service error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc, jwtHdl := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), svc, nil, jwtHdl)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/login", nil)

			res, err := h.LoginJWT(ctx, tc.req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_Edit(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) service.UserService
		req  dto.EditProfileRequest
		uc   jwtware.UserClaims

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "编辑成功",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().UpdateNonSensitiveInfo(gomock.Any(), domain.User{
					ID:       123,
					Nickname: "new_name",
					Birthday: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
					AboutMe:  "new about me",
				}).Return(nil)
				return svc
			},
			req: dto.EditProfileRequest{
				Nickname: "new_name",
				Birthday: "2000-01-01",
				AboutMe:  "new about me",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "更新成功",
			},
			wantErr: nil,
		},
		{
			name: "生日格式不对",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				return svc
			},
			req: dto.EditProfileRequest{
				Birthday: "invalid-date",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "生日格式不对",
			},
			wantErr: func() error {
				_, err := time.Parse(time.DateOnly, "invalid-date")
				return err
			}(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), svc, nil, nil)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/edit", nil)

			res, err := h.Edit(ctx, tc.req, tc.uc)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_SendSMSLoginCode(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) codeservice.CodeService
		req  dto.SendSMSCodeRequest

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "发送成功",
			mock: func(ctrl *gomock.Controller) codeservice.CodeService {
				svc := codemocks.NewMockCodeService(ctrl)
				svc.EXPECT().Send(gomock.Any(), "login", "12345678901").Return(nil)
				return svc
			},
			req: dto.SendSMSCodeRequest{
				Phone: "12345678901",
			},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "发送成功",
			},
			wantErr: nil,
		},
		{
			name: "未输入手机号码",
			mock: func(ctrl *gomock.Controller) codeservice.CodeService {
				svc := codemocks.NewMockCodeService(ctrl)
				return svc
			},
			req: dto.SendSMSCodeRequest{
				Phone: "",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "请输入手机号码",
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

			svc := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), nil, svc, nil)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/login_sms/code/send", nil)

			res, err := h.SendSMSLoginCode(ctx, tc.req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_LoginSMS(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService, jwtware.Handler)
		req  dto.SMSLoginRequest

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "登录成功",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService, jwtware.Handler) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)
				jwtHdl := jwtmocks.NewMockHandler(ctrl)

				codeSvc.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(true, nil)
				userSvc.EXPECT().FindOrCreate(gomock.Any(), "12345678901").Return(domain.User{ID: 123}, nil)
				jwtHdl.EXPECT().SetLoginToken(gomock.Any(), int64(123)).Return(jwtware.TokenPair{
					AccessToken:  "access_token",
					RefreshToken: "refresh_token",
				}, nil)

				return codeSvc, userSvc, jwtHdl
			},
			req: dto.SMSLoginRequest{
				Phone: "12345678901",
				Code:  "123456",
			},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "登录成功",
				Data: dto.SMSLoginResponse{
					AccessToken:  "access_token",
					RefreshToken: "refresh_token",
					User:         dto.ProfileResponse{ID: 123},
				},
			},
			wantErr: nil,
		},
		{
			name: "验证码不对",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService, jwtware.Handler) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)
				jwtHdl := jwtmocks.NewMockHandler(ctrl)

				codeSvc.EXPECT().Verify(gomock.Any(), "login", "12345678901", "123456").Return(false, nil)

				return codeSvc, userSvc, jwtHdl
			},
			req: dto.SMSLoginRequest{
				Phone: "12345678901",
				Code:  "123456",
			},
			wantResult: ginx.Result{
				Code: errs.UserCodeInvalid,
				Msg:  "验证码不对，请重新输入",
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

			codeSvc, userSvc, jwtHdl := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), userSvc, codeSvc, jwtHdl)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/login_sms", nil)

			res, err := h.LoginSMS(ctx, tc.req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_Profile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) service.UserService
		uc   jwtware.UserClaims

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "查询成功",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().FindById(gomock.Any(), int64(123)).Return(domain.User{
					Nickname: "test_user",
					Email:    "test@example.com",
					AboutMe:  "I am a tester",
					Birthday: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
					Avatar:   "avatar.jpg",
				}, nil)
				return svc
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "获取用户信息成功",
				Data: dto.ProfileResponse{
					Nickname: "test_user",
					Email:    "test@example.com",
					AboutMe:  "I am a tester",
					Birthday: "2000-01-01",
					Avatar:   "avatar.jpg",
				},
			},
			wantErr: nil,
		},
		{
			name: "查询失败",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().FindById(gomock.Any(), int64(123)).Return(domain.User{}, errors.New("db error"))
				return svc
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统错误",
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

			svc := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), svc, nil, nil)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("GET", "/users/profile", nil)

			res, err := h.Profile(ctx, tc.uc)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_ChangePassword(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		mock       func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler)
		req        dto.ChangePasswordRequest
		uc         jwtware.UserClaims
		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "修改密码成功",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				jwtHdl := jwtmocks.NewMockHandler(ctrl)
				svc.EXPECT().ChangePassword(gomock.Any(), int64(123), "OldPassword123!", "NewPassword123!").Return(nil)
				jwtHdl.EXPECT().ClearToken(gomock.Any()).Return(nil)
				return svc, jwtHdl
			},
			req: dto.ChangePasswordRequest{
				OldPassword:     "OldPassword123!",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "修改密码成功，请重新登录",
			},
			wantErr: nil,
		},
		{
			name: "旧密码错误",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().ChangePassword(gomock.Any(), int64(123), "OldPassword123!", "NewPassword123!").Return(service.ErrInvalidUserOrPassword)
				return svc, nil
			},
			req: dto.ChangePasswordRequest{
				OldPassword:     "OldPassword123!",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInvalidOrPassword,
				Msg:  "旧密码错误",
			},
			wantErr: service.ErrInvalidUserOrPassword,
		},
		{
			name: "两次输入密码不一致",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				return svc, nil
			},
			req: dto.ChangePasswordRequest{
				OldPassword:     "OldPassword123!",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword1234!",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "两次输入密码不同",
			},
			wantErr: nil,
		},
		{
			name: "新密码格式错误",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				return svc, nil
			},
			req: dto.ChangePasswordRequest{
				OldPassword:     "OldPassword123!",
				NewPassword:     "123",
				ConfirmPassword: "123",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "密码长度至少为6位",
			},
			wantErr: nil,
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) (service.UserService, jwtware.Handler) {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().ChangePassword(gomock.Any(), int64(123), "OldPassword123!", "NewPassword123!").Return(errors.New("service error"))
				return svc, nil
			},
			req: dto.ChangePasswordRequest{
				OldPassword:     "OldPassword123!",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统错误",
			},
			wantErr: errors.New("service error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc, jwtHdl := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), svc, nil, jwtHdl)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/change_password", nil)

			res, err := h.ChangePassword(ctx, tc.req, tc.uc)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_SendSMSResetPasswordCode(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) codeservice.CodeService
		req  dto.SendSMSResetPasswordCodeRequest

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "发送成功",
			mock: func(ctrl *gomock.Controller) codeservice.CodeService {
				svc := codemocks.NewMockCodeService(ctrl)
				svc.EXPECT().Send(gomock.Any(), "reset-password", "12345678901").Return(nil)
				return svc
			},
			req: dto.SendSMSResetPasswordCodeRequest{
				Phone: "12345678901",
			},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "发送成功",
			},
			wantErr: nil,
		},
		{
			name: "未输入手机号码或邮箱",
			mock: func(ctrl *gomock.Controller) codeservice.CodeService {
				svc := codemocks.NewMockCodeService(ctrl)
				return svc
			},
			req: dto.SendSMSResetPasswordCodeRequest{
				Phone: "",
				Email: "",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "请输入手机号码或邮箱",
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

			svc := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), nil, svc, nil)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/reset_password/code/send", nil)

			res, err := h.SendSMSResetPasswordCode(ctx, tc.req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_ResetPassword(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService)
		req  dto.ResetPasswordRequest

		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "重置密码成功",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)

				codeSvc.EXPECT().Verify(gomock.Any(), "reset-password", "test@example.com", "123456").Return(true, nil)
				userSvc.EXPECT().ResetPasswordByEmail(gomock.Any(), "test@example.com", "NewPassword123!").Return(nil)

				return codeSvc, userSvc
			},
			req: dto.ResetPasswordRequest{
				Phone:           "12345678901",
				Code:            "123456",
				Password:        "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
				Email:           "test@example.com",
			},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "重置密码成功",
			},
			wantErr: nil,
		},
		{
			name: "验证码错误",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)

				codeSvc.EXPECT().Verify(gomock.Any(), "reset-password", "test@example.com", "123456").Return(false, nil)

				return codeSvc, userSvc
			},
			req: dto.ResetPasswordRequest{
				Phone:           "12345678901",
				Code:            "123456",
				Password:        "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
				Email:           "test@example.com",
			},
			wantResult: ginx.Result{
				Code: errs.UserCodeInvalid,
				Msg:  "验证码不对，请重新输入",
			},
			wantErr: nil,
		},
		{
			name: "两次输入密码不一致",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)
				return codeSvc, userSvc
			},
			req: dto.ResetPasswordRequest{
				Phone:           "12345678901",
				Code:            "123456",
				Password:        "NewPassword123!",
				ConfirmPassword: "NewPassword1234!",
				Email:           "test@example.com",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "两次输入密码不同",
			},
			wantErr: nil,
		},
		{
			name: "新密码格式错误",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)
				return codeSvc, userSvc
			},
			req: dto.ResetPasswordRequest{
				Phone:           "12345678901",
				Code:            "123456",
				Password:        "123",
				ConfirmPassword: "123",
				Email:           "test@example.com",
			},
			wantResult: ginx.Result{
				Code: errs.UserInvalidInput,
				Msg:  "密码长度至少为6位",
			},
			wantErr: nil,
		},
		{
			name: "系统错误",
			mock: func(ctrl *gomock.Controller) (codeservice.CodeService, service.UserService) {
				codeSvc := codemocks.NewMockCodeService(ctrl)
				userSvc := svcmocks.NewMockUserService(ctrl)

				codeSvc.EXPECT().Verify(gomock.Any(), "reset-password", "test@example.com", "123456").Return(true, nil)
				userSvc.EXPECT().ResetPasswordByEmail(gomock.Any(), "test@example.com", "NewPassword123!").Return(errors.New("service error"))

				return codeSvc, userSvc
			},
			req: dto.ResetPasswordRequest{
				Phone:           "12345678901",
				Code:            "123456",
				Password:        "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
				Email:           "test@example.com",
			},
			wantResult: ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统错误",
			},
			wantErr: errors.New("service error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			codeSvc, userSvc := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), userSvc, codeSvc, nil)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/users/reset_password", nil)

			res, err := h.ResetPassword(ctx, tc.req)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}

func TestUserHandler_UploadAvatar(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		mock       func(ctrl *gomock.Controller) service.UserService
		uc         jwtware.UserClaims
		wantResult ginx.Result
		wantErr    error
	}{
		{
			name: "头像上传成功",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().UpdateAvatarPath(gomock.Any(), gomock.Eq(int64(123)), gomock.Any()).Return(nil)
				return svc
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: http.StatusOK,
				Msg:  "头像上传成功",
				Data: map[string]string{"avatar": "storage/uploads/avatars/123_test_avatar.jpg"},
			},
			wantErr: nil,
		},
		{
			name: "头像上传失败_服务错误",
			mock: func(ctrl *gomock.Controller) service.UserService {
				svc := svcmocks.NewMockUserService(ctrl)
				svc.EXPECT().UpdateAvatarPath(gomock.Any(), gomock.Eq(int64(123)), gomock.Any()).Return(errors.New("service error"))
				return svc
			},
			uc: jwtware.UserClaims{Uid: 123},
			wantResult: ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "头像上传失败",
			},
			wantErr: errors.New("service error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tc.mock(ctrl)
			h := NewUserHandler(logger.NewNopLogger(), svc, nil, nil)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			// Create a multipart form with a dummy file
			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("avatar", "test_avatar.jpg")
			_, _ = part.Write([]byte("dummy image content"))
			writer.Close()

			ctx.Request = httptest.NewRequest("POST", "/users/avatar/upload", body)
			ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

			res, err := h.UploadAvatar(ctx, tc.uc)

			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantResult, res)
		})
	}
}
