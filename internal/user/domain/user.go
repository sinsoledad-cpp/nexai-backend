package domain

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID         int64
	Email      string
	Password   string
	Nickname   string
	Avatar     string    // 头像
	Birthday   time.Time // YYYY-MM-DD
	AboutMe    string
	Phone      string
	Ctime      time.Time // UTC 0 的时区
	WechatInfo WechatInfo
	//Addr Address
}
type WechatInfo struct {
	UnionID string
	OpenID  string
}

func (u *User) VerifyPassword(inputPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(inputPassword))
	return err == nil
}
