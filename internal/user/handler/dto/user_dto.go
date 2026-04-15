package dto

type SignUpRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string          `json:"accessToken"`
	RefreshToken string          `json:"refreshToken"`
	User         ProfileResponse `json:"user"`
}

type ProfileResponse struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	AboutMe  string `json:"aboutMe"`
	Birthday string `json:"birthday"`
	Avatar   string `json:"avatar"`
}

type EditProfileRequest struct {
	Nickname string `json:"nickname"`
	Birthday string `json:"birthday"`
	AboutMe  string `json:"aboutMe"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"accessToken"`
}

type SMSLoginRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
}

type SMSLoginResponse struct {
	AccessToken  string          `json:"accessToken"`
	RefreshToken string          `json:"refreshToken"`
	User         ProfileResponse `json:"user"`
}

type SendSMSCodeRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
}

type SendSMSResetPasswordCodeRequest struct {
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	Code            string `json:"code" binding:"required"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword     string `json:"oldPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type ChangeEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code"`
}

type ChangePhoneRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
	Code  string `json:"code"`
}

type SendChangeEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type SendChangePhoneCodeRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
}
