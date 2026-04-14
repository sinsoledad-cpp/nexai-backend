package handler

import (
	"errors"
	"fmt"
	"net/http"
	codeservice "nexai-backend/internal/code/service"
	jwtware "nexai-backend/internal/common/jwt"
	"nexai-backend/internal/common/router"
	"nexai-backend/internal/user/domain"
	"nexai-backend/internal/user/handler/dto"
	"nexai-backend/internal/user/handler/errs"
	userservice "nexai-backend/internal/user/service"
	"nexai-backend/pkg/ginx"
	"nexai-backend/pkg/logger"
	"time"

	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var _ router.Handler = (*UserHandler)(nil)

const (
	emailRegexPattern    = "(?i)^[A-Z0-9_!#$%&'*+/=?`{|}~^.-]+@[A-Z0-9.-]+$"
	passwordRegexPattern = `^.{1,}$` // 允许任意密码，长度至少为1
	bizLogin             = "login"
	bizResetPassword     = "reset-password"
)

type UserHandler struct {
	log              logger.Logger
	userSvc          userservice.UserService
	codeSvc          codeservice.CodeService
	jwtHdl           jwtware.Handler
	emailRegexExp    *regexp.Regexp
	passwordRegexExp *regexp.Regexp
}

func NewUserHandler(log logger.Logger, userSvc userservice.UserService, codeSvc codeservice.CodeService, jwtHdl jwtware.Handler) *UserHandler {
	return &UserHandler{
		log:              log,
		userSvc:          userSvc,
		codeSvc:          codeSvc,
		jwtHdl:           jwtHdl,
		emailRegexExp:    regexp.MustCompile(emailRegexPattern, regexp.None),
		passwordRegexExp: regexp.MustCompile(passwordRegexPattern, regexp.None),
	}
}

func (u *UserHandler) RegisterRoutes(e *gin.Engine) {
	// API 版本控制
	v1 := e.Group("/v1")

	// 用户相关路由
	users := v1.Group("/users")

	// 认证相关
	users.POST("/signup", ginx.WrapBody(u.SignUp))
	users.POST("/login", ginx.WrapBody(u.LoginJWT))
	users.DELETE("/token", ginx.Wrap(u.LogoutJWT))
	users.PUT("/token", ginx.Wrap(u.RefreshToken))

	// 用户信息相关
	users.POST("/avatar", ginx.WrapClaims(u.UploadAvatar))
	users.PUT("/profile", ginx.WrapBodyAndClaims(u.Edit))
	users.GET("/profile", ginx.WrapClaims(u.Profile))

	// 短信验证码相关
	users.POST("/verification-codes/login", ginx.WrapBody(u.SendSMSLoginCode))
	users.POST("/login/sms", ginx.WrapBody(u.LoginSMS))
	users.POST("/verification-codes/reset-password", ginx.WrapBody(u.SendSMSResetPasswordCode))

	// 密码相关
	users.POST("/password/reset", ginx.WrapBody(u.ResetPassword))
	users.PUT("/password", ginx.WrapBodyAndClaims(u.ChangePassword))
}

func (u *UserHandler) SignUp(ctx *gin.Context, req dto.SignUpRequest) (ginx.Result, error) {
	// 校验客户端输入
	isEmail, err := u.emailRegexExp.MatchString(req.Email)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	if !isEmail {

		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "邮箱格式错误",
		}, nil
	}
	if req.Password != req.ConfirmPassword {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "两次输入密码不同",
		}, nil
	}
	isPassword, err := u.passwordRegexExp.MatchString(req.Password)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "系统错误",
		}, err
	}
	if !isPassword {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "密码不能为空",
		}, nil
	}

	// 业务逻辑
	err = u.userSvc.Signup(ctx.Request.Context(), domain.User{Email: req.Email, Password: req.ConfirmPassword})
	if errors.Is(err, userservice.ErrDuplicateEmail) {
		u.log.Warn("用户邮箱冲突", logger.Error(err))
		return ginx.Result{
			Code: errs.UserDuplicateEmail,
			Msg:  "邮箱冲突",
		}, err
	}
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}

	return ginx.Result{
		Code: http.StatusCreated,
		Msg:  "注册成功",
	}, nil
}

func (u *UserHandler) LoginJWT(ctx *gin.Context, req dto.LoginRequest) (ginx.Result, error) {
	user, err := u.userSvc.Login(ctx, req.Email, req.Password)
	switch {
	case err == nil:
		err = u.jwtHdl.SetLoginToken(ctx, user.ID)
		if err != nil {
			return ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统错误",
			}, err
		}
		return ginx.Result{
			Code: http.StatusOK,
			Msg:  "登录成功",
		}, nil
	case errors.Is(err, userservice.ErrInvalidUserOrPassword):
		return ginx.Result{
			Code: errs.UserInvalidOrPassword,
			Msg:  "用户名或者密码错误",
		}, err
	default:
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
}

func (u *UserHandler) LogoutJWT(ctx *gin.Context) (ginx.Result, error) {
	err := u.jwtHdl.ClearToken(ctx)
	if err != nil {
		ctx.JSON(http.StatusOK, ginx.Result{Code: http.StatusInternalServerError, Msg: "系统错误"})
		return ginx.Result{
			Code: http.StatusInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "退出登录成功",
	}, nil
}

func (u *UserHandler) RefreshToken(ctx *gin.Context) (ginx.Result, error) {
	// 假定长 token 也放在这里
	tokenStr := ctx.GetHeader("X-Refresh-Token")

	var rc jwtware.RefreshClaims
	token, err := jwt.ParseWithClaims(tokenStr, &rc, func(token *jwt.Token) (interface{}, error) {
		return jwtware.RefreshTokenKey, nil
	})
	// 这边要保持和登录校验一直的逻辑，即返回 401 响应
	if err != nil || token == nil || !token.Valid {
		return ginx.Result{
			Code: http.StatusUnauthorized,
			Msg:  "登录已过期，请重新登录",
		}, err
	}

	// 校验 ssid
	err = u.jwtHdl.CheckSession(ctx, rc.Ssid)
	if err != nil {
		// 如果是会话不存在的业务错误，返回 401
		if errors.Is(err, jwtware.ErrSessionNotFound) {
			return ginx.Result{
				Code: http.StatusUnauthorized,
				Msg:  "会话已过期，请重新登录",
			}, err
		}
		// 系统错误或者用户已经主动退出登录了
		// 这里也可以考虑说，如果在 Redis 已经崩溃的时候，
		// 就不要去校验是不是已经主动退出登录了。
		//ctx.AbortWithStatus(http.StatusUnauthorized)
		return ginx.Result{
			Code: http.StatusInternalServerError,
			Msg:  "系统错误",
		}, err
	}

	err = u.jwtHdl.SetJWTToken(ctx, rc.Uid, rc.Ssid)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统内部错误",
		}, err
	}
	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "刷新成功",
	}, nil
}

func (u *UserHandler) UploadAvatar(ctx *gin.Context, uc jwtware.UserClaims) (ginx.Result, error) {
	file, err := ctx.FormFile("avatar")
	if err != nil {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "无法获取头像文件",
		}, err
	}

	// 假设头像存储在本地的 uploads/avatars 目录下
	// 实际项目中应该使用对象存储服务，如 AWS S3, Aliyun OSS 等
	// 为了简化，这里仅模拟存储
	avatarPath := fmt.Sprintf("uploads/avatars/%d_%s", uc.Uid, file.Filename)
	err = ctx.SaveUploadedFile(file, avatarPath)
	if err != nil {
		u.log.Error("保存头像文件失败", logger.Error(err))
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "头像上传失败",
		}, err
	}

	err = u.userSvc.UpdateAvatarPath(ctx, uc.Uid, avatarPath)
	if err != nil {
		// 如果服务层更新失败，理论上需要删除已经上传的文件
		u.log.Error("更新用户头像路径失败", logger.Error(err))
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "头像上传失败",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "头像上传成功",
	}, nil
}

func (u *UserHandler) Profile(ctx *gin.Context, uc jwtware.UserClaims) (ginx.Result, error) {
	user, err := u.userSvc.FindById(ctx, uc.Uid)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "获取用户信息成功",
		Data: dto.ProfileResponse{
			Nickname: user.Nickname,
			Email:    user.Email,
			AboutMe:  user.AboutMe,
			Birthday: user.Birthday.Format(time.DateOnly),
			Avatar:   user.Avatar,
		},
	}, nil
}

func (u *UserHandler) Edit(ctx *gin.Context, req dto.EditProfileRequest, uc jwtware.UserClaims) (ginx.Result, error) {
	// 嵌入一段刷新过期时间的代码
	//sess := sessions.Default(ctx)
	//sess.Get("uid")
	// 用户输入不对
	birthday, err := time.Parse(time.DateOnly, req.Birthday)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "生日格式不对",
		}, err
	}
	err = u.userSvc.UpdateNonSensitiveInfo(ctx, domain.User{
		ID:       uc.Uid,
		Nickname: req.Nickname,
		Birthday: birthday,
		AboutMe:  req.AboutMe,
	})
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "上传成功",
	}, nil
}

func (u *UserHandler) SendSMSLoginCode(ctx *gin.Context, req dto.SendSMSCodeRequest) (ginx.Result, error) {
	// 你这边可以校验 Req
	if req.Phone == "" {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "请输入手机号码",
		}, nil
	}
	err := u.codeSvc.Send(ctx, bizLogin, req.Phone)
	switch {
	case err == nil:
		return ginx.Result{
			Code: http.StatusOK,
			Msg:  "发送成功",
		}, nil
	case errors.Is(err, codeservice.ErrCodeSendTooMany):
		// 事实上，防不住有人不知道怎么触发了
		// 少数这种错误，是可以接受的
		// 但是频繁出现，就代表有人在搞你的系统
		return ginx.Result{
			Code: errs.UserCodeSendTooMany,
			Msg:  "短信发送太频繁，请稍后再试",
		}, nil
	default:
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
}

func (u *UserHandler) LoginSMS(ctx *gin.Context, req dto.SMSLoginRequest) (ginx.Result, error) {
	ok, err := u.codeSvc.Verify(ctx, bizLogin, req.Phone, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, codeservice.ErrCodeVerifyTooMany):
			return ginx.Result{
				Code: errs.UserCodeVerifyTooMany,
				Msg:  "验证码验证次数太多，请稍后再试",
			}, nil
		case errors.Is(err, codeservice.ErrCodeExpired):
			return ginx.Result{
				Code: errs.UserCodeExpired,
				Msg:  "验证码已过期",
			}, nil
		default:
			return ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统异常",
			}, err
		}
	}
	if !ok {
		return ginx.Result{
			Code: errs.UserCodeInvalid,
			Msg:  "验证码不对，请重新输入",
		}, nil
	}
	user, err := u.userSvc.FindOrCreate(ctx, req.Phone)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	err = u.jwtHdl.SetLoginToken(ctx, user.ID)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "登录成功",
	}, nil
}

func (u *UserHandler) SendSMSResetPasswordCode(ctx *gin.Context, req dto.SendSMSResetPasswordCodeRequest) (ginx.Result, error) {
	if req.Phone == "" && req.Email == "" {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "请输入手机号码或邮箱",
		}, nil
	}
	target := req.Phone
	if req.Email != "" {
		target = req.Email
	}
	err := u.codeSvc.Send(ctx, bizResetPassword, target)
	switch {
	case err == nil:
		return ginx.Result{
			Code: http.StatusOK,
			Msg:  "发送成功",
		}, nil
	case errors.Is(err, codeservice.ErrCodeSendTooMany):
		return ginx.Result{
			Code: errs.UserCodeSendTooMany,
			Msg:  "短信发送太频繁，请稍后再试",
		}, nil
	default:
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
}

func (u *UserHandler) ResetPassword(ctx *gin.Context, req dto.ResetPasswordRequest) (ginx.Result, error) {
	if req.Phone == "" && req.Email == "" {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "请输入手机号码或邮箱",
		}, nil
	}
	if req.Password != req.ConfirmPassword {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "两次输入密码不同",
		}, nil
	}
	isPassword, err := u.passwordRegexExp.MatchString(req.Password)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "系统错误",
		}, err
	}
	if !isPassword {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "密码不能为空",
		}, nil
	}

	target := req.Phone
	if req.Email != "" {
		target = req.Email
	}

	ok, err := u.codeSvc.Verify(ctx, bizResetPassword, target, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, codeservice.ErrCodeVerifyTooMany):
			return ginx.Result{
				Code: errs.UserCodeVerifyTooMany,
				Msg:  "验证码验证次数太多，请稍后再试",
			}, nil
		case errors.Is(err, codeservice.ErrCodeExpired):
			return ginx.Result{
				Code: errs.UserCodeExpired,
				Msg:  "验证码已过期",
			}, nil
		default:
			return ginx.Result{
				Code: errs.UserInternalServerError,
				Msg:  "系统异常",
			}, err
		}
	}
	if !ok {
		return ginx.Result{
			Code: errs.UserCodeInvalid,
			Msg:  "验证码不对，请重新输入",
		}, nil
	}
	if req.Email != "" {
		err = u.userSvc.ResetPasswordByEmail(ctx, req.Email, req.Password)
	} else {
		err = u.userSvc.ResetPasswordByPhone(ctx, req.Phone, req.Password)
	}
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "重置密码成功",
	}, nil
}

func (u *UserHandler) ChangePassword(ctx *gin.Context, req dto.ChangePasswordRequest, uc jwtware.UserClaims) (ginx.Result, error) {
	if req.NewPassword != req.ConfirmPassword {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "两次输入密码不同",
		}, nil
	}

	isPassword, err := u.passwordRegexExp.MatchString(req.NewPassword)
	if err != nil {
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}
	if !isPassword {
		return ginx.Result{
			Code: errs.UserInvalidInput,
			Msg:  "密码不能为空",
		}, nil
	}

	err = u.userSvc.ChangePassword(ctx, uc.Uid, req.OldPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, userservice.ErrInvalidUserOrPassword) {
			return ginx.Result{
				Code: errs.UserInvalidOrPassword,
				Msg:  "旧密码错误",
			}, err
		}
		return ginx.Result{
			Code: errs.UserInternalServerError,
			Msg:  "系统错误",
		}, err
	}

	return ginx.Result{
		Code: http.StatusOK,
		Msg:  "修改密码成功",
	}, nil
}
