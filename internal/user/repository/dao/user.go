package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID       int64          `gorm:"primaryKey,autoIncrement"`
	Email    sql.NullString `gorm:"uniqueIndex;size:255"` //Email    *string // 代表这是一个可以为 NULL 的列
	Password string
	Nickname string         `gorm:"type=varchar(128)"`
	Birthday sql.NullInt64  // YYYY-MM-DD
	Avatar   string         `gorm:"type=varchar(1024)"` // 头像
	AboutMe  string         `gorm:"type=varchar(4096)"`
	Phone    sql.NullString `gorm:"uniqueIndex;size:20"` // 代表这是一个可以为 NULL 的列
	Ctime    int64          // 创建时间 // 时区，UTC 0 的毫秒数
	Utime    int64          // 更新时间
	// json 存储
	//Addr string
}

var (
	ErrDuplicateEmail = errors.New("邮箱冲突")
	ErrDuplicatePhone = errors.New("手机号冲突")
	ErrRecordNotFound = gorm.ErrRecordNotFound
)

//go:generate mockgen -source=./user.go -package=mocks -destination=./mocks/user_mock.go UserDAO
type UserDAO interface {
	Insert(ctx context.Context, user User) error
	FindByEmail(ctx context.Context, email string) (User, error)
	UpdateAvatar(ctx context.Context, id int64, avatar string) error
	UpdateById(ctx context.Context, entity User) error
	FindById(ctx context.Context, uid int64) (User, error)
	FindByPhone(ctx context.Context, phone string) (User, error)
}

type GORMUserDAO struct {
	db *gorm.DB
}

func NewGORMUserDAO(db *gorm.DB) UserDAO {
	return &GORMUserDAO{
		db: db,
	}
}
func (g *GORMUserDAO) Insert(ctx context.Context, user User) error {
	now := time.Now().UnixMilli()
	user.Ctime = now
	user.Utime = now
	err := g.db.WithContext(ctx).Create(&user).Error
	if err != nil {
		// 检查是否是唯一约束冲突
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			if strings.Contains(err.Error(), "email") {
				return ErrDuplicateEmail
			}
			if strings.Contains(err.Error(), "phone") {
				return ErrDuplicatePhone
			}
			return ErrDuplicateEmail
		}
	}
	return err
}

func (g *GORMUserDAO) FindByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := g.db.WithContext(ctx).Where("email=?", email).First(&u).Error
	return u, err
}

func (g *GORMUserDAO) UpdateAvatar(ctx context.Context, id int64, avatar string) error {
	return g.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(
		map[string]any{
			"avatar": avatar,
			"utime":  time.Now().UnixMilli(),
		}).Error
}
func (g *GORMUserDAO) UpdateById(ctx context.Context, user User) error {
	now := time.Now().UnixMilli()
	updates := map[string]any{
		"utime": now,
	}
	if user.Nickname != "" {
		updates["nickname"] = user.Nickname
	}
	if user.Birthday.Valid {
		updates["birthday"] = user.Birthday
	}
	if user.AboutMe != "" {
		updates["about_me"] = user.AboutMe
	}
	if user.Password != "" {
		updates["password"] = user.Password
	}
	if user.Email.Valid {
		updates["email"] = user.Email
	}
	if user.Phone.Valid {
		updates["phone"] = user.Phone
	}
	return g.db.WithContext(ctx).Model(&user).Where("id = ?", user.ID).
		Updates(updates).Error
}
func (g *GORMUserDAO) FindById(ctx context.Context, uid int64) (User, error) {
	var res User
	err := g.db.WithContext(ctx).Where("id = ?", uid).First(&res).Error
	return res, err
}

func (g *GORMUserDAO) FindByPhone(ctx context.Context, phone string) (User, error) {
	var res User
	err := g.db.WithContext(ctx).Where("phone = ?", phone).First(&res).Error
	return res, err
}
