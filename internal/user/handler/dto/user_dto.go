package dto

type SignUpRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

type ProfileResponse struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	AboutMe  string `json:"aboutMe"`
	Birthday string `json:"birthday"`
	Avatar   string `json:"avatar"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type EditProfileRequest struct {
	Nickname string `json:"nickname"`
	Birthday string `json:"birthday"`
	AboutMe  string `json:"aboutMe"`
}

type SMSLoginRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
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
