package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"nexai-backend/internal/user/domain"
	"nexai-backend/internal/user/repository/cache"
	"nexai-backend/internal/user/repository/dao"
	"nexai-backend/pkg/logger"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	ErrDuplicatePhone  = dao.ErrDuplicatePhone
	ErrDuplicateEmail  = dao.ErrDuplicateEmail
	ErrDuplicateWechat = dao.ErrDuplicateWechat
	ErrUserNotFound    = dao.ErrRecordNotFound
)

//go:generate mockgen -source=./user.go -package=mocks -destination=./mocks/user_mock.go UserRepository
type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	UpdateAvatar(ctx context.Context, id int64, avatar string) error
	UpdateNonZeroFields(ctx context.Context, user domain.User) error
	FindByPhone(ctx context.Context, phone string) (domain.User, error)

	FindById(ctx context.Context, uID int64) (domain.User, error)
	FindByWechat(ctx context.Context, openID string) (domain.User, error)
}

type CachedUserRepository struct {
	dao   dao.UserDAO
	cache cache.UserCache
	l     logger.Logger
	g     singleflight.Group //不需要初始化，零值直接可用
}

func NewCachedUserRepository(userDAO dao.UserDAO, userCache cache.UserCache, l logger.Logger) UserRepository {
	return &CachedUserRepository{
		dao:   userDAO,
		cache: userCache,
		l:     l,
	}
}

func (c *CachedUserRepository) Create(ctx context.Context, user domain.User) error {
	return c.dao.Insert(ctx, c.toEntity(user))
}
func (c *CachedUserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := c.dao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	return c.toDomain(u), nil
}

func (c *CachedUserRepository) UpdateAvatar(ctx context.Context, id int64, avatar string) error {
	// 更新数据库
	err := c.dao.UpdateAvatar(ctx, id, avatar)
	if err != nil {
		return err
	}
	// 操作缓存：这里选择直接删除缓存，让下一次查询重新加载
	return c.cache.Delete(ctx, id)
}

func (c *CachedUserRepository) UpdateNonZeroFields(ctx context.Context, user domain.User) error {
	// 更新 DB 之后，删除
	err := c.dao.UpdateById(ctx, c.toEntity(user))
	if err != nil {
		return err
	}
	// 延迟一秒再次删除（延时双删）
	time.AfterFunc(time.Second, func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := c.cache.Delete(bgCtx, user.ID); err != nil {
			// 这里因为是在 goroutine 里，建议记录一条 Error 日志
			// fmt.Printf("延时双删失败: %v\n", err)
			c.l.Error("延时双删失败", logger.Error(err), logger.Int64("uid", user.ID))
		}
	})
	return c.cache.Delete(ctx, user.ID)
}
func (c *CachedUserRepository) FindByPhone(ctx context.Context, phone string) (domain.User, error) {
	u, err := c.dao.FindByPhone(ctx, phone)
	if err != nil {
		return domain.User{}, err
	}
	return c.toDomain(u), nil
}

// FindById 获取用户详情（带缓存击穿防护）
func (c *CachedUserRepository) FindById(ctx context.Context, uid int64) (domain.User, error) {
	// 1. 先查询缓存
	du, err := c.cache.Get(ctx, uid)
	if err == nil {
		return du, nil
	}

	// 2. 如果是其他错误（例如 Redis 挂了或连接超时），根据你的策略决定。
	// 这里保留你原有的逻辑：如果只是 Key 不存在，才去查库；如果是系统错误，则直接返回错误。
	// (在某些高可用场景下，Redis 挂了你可能希望降级去查库，那可以去掉这个 if 判断，直接往下走)
	if !errors.Is(err, cache.ErrKeyNotExist) {
		return domain.User{}, err
	}

	// 3. 缓存未命中，使用 singleflight 防止击穿
	// key 需要具备唯一性，通常格式为 "业务前缀:参数"
	key := fmt.Sprintf("user:id:%d", uid)

	// Do 方法确保在同一时刻，针对同一个 key，函数内部的逻辑只会被执行一次
	// 后续并发进来的请求会等待第一个请求返回，并共享结果
	val, err, _ := c.g.Do(key, func() (interface{}, error) {
		// 3.1 查数据库
		u, err := c.dao.FindById(ctx, uid)
		if err != nil {
			return domain.User{}, err
		}

		// 3.2 转换为领域对象
		du := c.toDomain(u)

		// 3.3 回写缓存
		// 注意：这里如果回写失败，通常只记录日志，不应影响主业务流程返回
		err = c.cache.Set(ctx, du)
		if err != nil {
			// 建议：此处加上日志记录
			// log.Println("回写用户缓存失败", err)
			// fmt.Printf("回写用户缓存失败: %v\n", err)
			c.l.Warn("回写用户缓存失败", logger.Error(err), logger.Int64("uid", uid))
		}

		return du, nil
	})

	if err != nil {
		return domain.User{}, err
	}

	// 4. 类型断言
	// 【安全优化】使用 comma-ok 断言
	// 如果转换失败，ok 为 false，不会 panic
	user, ok := val.(domain.User)
	if !ok {
		// 这种情况理论上不应该发生，除非 singleflight 内部的函数返回类型被改了
		// 这里应该记录一条 Error 级别的日志，提示开发者检查代码
		// log.Error("singleflight type assertion failed", logger.String("key", key))
		c.l.Error("singleflight type assertion failed", logger.String("key", key))
		return domain.User{}, errors.New("系统内部错误: 类型转换失败")
	}

	return user, nil
}

//	func (c *CachedUserRepository) FindById(ctx context.Context, uid int64) (domain.User, error) {
//		du, err := c.cache.Get(ctx, uid)
//		switch {
//		case err == nil: // 只要 err 为 nil，就返回
//			return du, nil
//		case errors.Is(err, cache.ErrKeyNotExist):
//			u, err := c.dao.FindById(ctx, uid)
//			if err != nil {
//				return domain.User{}, err
//			}
//			du = c.toDomain(u)
//			//go func() {
//			//	err = repo.cache.Set(ctx, du)
//			//	if err != nil {
//			//		log.Println(err)
//			//	}
//			//}()
//			err = c.cache.Set(ctx, du)
//			if err != nil {
//				// 网络崩了，也可能是 redis 崩了
//			}
//			return du, nil
//		default:
//			// 接近降级的写法
//			return domain.User{}, err
//		}
//	}
func (c *CachedUserRepository) FindByWechat(ctx context.Context, openID string) (domain.User, error) {
	ue, err := c.dao.FindByWechat(ctx, openID)
	if err != nil {
		return domain.User{}, err
	}
	return c.toDomain(ue), nil
}

func (c *CachedUserRepository) toEntity(user domain.User) dao.User {
	return dao.User{
		ID: user.ID,
		Email: sql.NullString{
			String: user.Email,
			Valid:  user.Email != "",
		},
		Phone: sql.NullString{
			String: user.Phone,
			Valid:  user.Phone != "",
		},
		Password: user.Password,
		Birthday: sql.NullInt64{
			Int64: user.Birthday.UnixMilli(),
			Valid: !user.Birthday.IsZero(), // 表示这个值是有效的，不是 NULL
		},
		Avatar: user.Avatar,
		WechatUnionId: sql.NullString{
			String: user.WechatInfo.UnionID,
			Valid:  user.WechatInfo.UnionID != "",
		},
		WechatOpenId: sql.NullString{
			String: user.WechatInfo.OpenID,
			Valid:  user.WechatInfo.OpenID != "",
		},
		AboutMe:  user.AboutMe,
		Nickname: user.Nickname,
	}
}
func (c *CachedUserRepository) toDomain(u dao.User) domain.User {
	var birthday time.Time
	// 检查从数据库取出的 birthday 是否有效
	if u.Birthday.Valid {
		birthday = time.UnixMilli(u.Birthday.Int64)
	}
	return domain.User{
		ID:       u.ID,
		Email:    u.Email.String,
		Phone:    u.Phone.String,
		Password: u.Password,
		AboutMe:  u.AboutMe,
		Nickname: u.Nickname,
		Birthday: birthday,
		Avatar:   u.Avatar,
		Ctime:    time.UnixMilli(u.Ctime),
		WechatInfo: domain.WechatInfo{
			OpenID:  u.WechatOpenId.String,
			UnionID: u.WechatUnionId.String,
		},
	}
}
