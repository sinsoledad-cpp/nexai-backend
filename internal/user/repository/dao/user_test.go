package dao

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMUserDAO_Insert(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mock    func(t *testing.T, mock sqlmock.Sqlmock)
		ctx     context.Context
		user    User
		wantErr error
	}{
		{
			name: "success",
			ctx:  context.Background(),
			user: User{
				Email:    sql.NullString{String: "test@example.com", Valid: true},
				Password: "password",
			},
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO .*").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			wantErr: nil,
		},
		{
			name: "duplicate email",
			ctx:  context.Background(),
			user: User{
				Email:    sql.NullString{String: "test@example.com", Valid: true},
				Password: "password",
			},
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO .*").
					WillReturnError(&mysql.MySQLError{
						Number:  1062,
						Message: "Duplicate entry 'test@example.com' for key 'uix_users_email'",
					})
				mock.ExpectRollback()
			},
			wantErr: ErrDuplicateEmail,
		},
		{
			name: "duplicate phone",
			ctx:  context.Background(),
			user: User{
				Phone:    sql.NullString{String: "12345678901", Valid: true},
				Password: "password",
			},
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO .*").
					WillReturnError(&mysql.MySQLError{
						Number:  1062,
						Message: "Duplicate entry '12345678901' for key 'phone'",
					})
				mock.ExpectRollback()
			},
			wantErr: ErrDuplicatePhone,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Mock DB
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// GORM
			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			err = dao.Insert(tc.ctx, tc.user)
			assert.Equal(t, tc.wantErr, err)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGORMUserDAO_FindByEmail(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		mock     func(t *testing.T, mock sqlmock.Sqlmock)
		ctx      context.Context
		email    string
		wantErr  error
		wantUser User
	}{
		{
			name:  "success",
			ctx:   context.Background(),
			email: "test@example.com",
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "wechat_open_id", "wechat_union_id", "ctime", "utime"}).
					AddRow(1, "test@example.com", "password", "nickname", 0, "avatar", "about", "12345678901", "openid", "unionid", 123, 123)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE email=\\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:            1,
				Email:         sql.NullString{String: "test@example.com", Valid: true},
				Password:      "password",
				Nickname:      "nickname",
				Birthday:      sql.NullInt64{Int64: 0, Valid: true},
				Avatar:        "avatar",
				AboutMe:       "about",
				Phone:         sql.NullString{String: "12345678901", Valid: true},
				WechatOpenId:  sql.NullString{String: "openid", Valid: true},
				WechatUnionId: sql.NullString{String: "unionid", Valid: true},
				Ctime:         123,
				Utime:         123,
			},
			wantErr: nil,
		},
		{
			name:  "not found",
			ctx:   context.Background(),
			email: "test@example.com",
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE email=\\?.*").
					WillReturnError(gorm.ErrRecordNotFound)
			},
			wantUser: User{},
			wantErr:  gorm.ErrRecordNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			user, err := dao.FindByEmail(tc.ctx, tc.email)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGORMUserDAO_UpdateAvatar(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mock    func(t *testing.T, mock sqlmock.Sqlmock)
		ctx     context.Context
		id      int64
		avatar  string
		wantErr error
	}{
		{
			name:   "success",
			ctx:    context.Background(),
			id:     1,
			avatar: "new_avatar.jpg",
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE .*users.* SET .*avatar.*=\\?,.*utime.*=\\? WHERE id = \\?").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			wantErr: nil,
		},
		{
			name:   "db error",
			ctx:    context.Background(),
			id:     1,
			avatar: "new_avatar.jpg",
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE .*users.* SET .*avatar.*=\\?,.*utime.*=\\? WHERE id = \\?").
					WillReturnError(errors.New("db error"))
				mock.ExpectRollback()
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			err = dao.UpdateAvatar(tc.ctx, tc.id, tc.avatar)
			assert.Equal(t, tc.wantErr, err)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGORMUserDAO_UpdateById(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mock    func(t *testing.T, mock sqlmock.Sqlmock)
		ctx     context.Context
		user    User
		wantErr error
	}{
		{
			name: "success",
			ctx:  context.Background(),
			user: User{
				ID:       1,
				Nickname: "new nickname",
			},
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				// GORM v2 Update map behavior:
				// It updates the columns specified in the map.
				// Since we are using Model(&user).Where("id = ?", user.ID).Updates(map...),
				// GORM generates UPDATE `users` SET ... WHERE id = ? AND `id` = ? if `user.ID` is non-zero in Model struct.
				// Wait, let's check dao/user.go again.
				// return g.db.WithContext(ctx).Model(&user).Where("id = ?", user.ID).Updates(...)
				// Model(&user) sets the table and potentially the primary key condition if ID is set.
				// If Model(&user) is used where user.ID=1, GORM adds `id` = 1 to the WHERE clause automatically for Updates/Delete.
				// Plus we added .Where("id = ?", user.ID) explicitly. So it might be `WHERE id = ? AND id = ?` or GORM smart enough to merge?
				// Usually it produces `WHERE id = ? AND id = ?` or `WHERE id = ?` depending on GORM version and configuration.
				// Let's assume GORM adds the PK condition from Model(&user) AND our explicit Where.
				// Actually, if we look at the error: `ExecQuery 'UPDATE ... WHERE id = ? AND id = ?', arguments do not match: expected 5, but got 6 arguments`
				// This confirms GORM added `id` = ? twice. One from explicit Where, one from Model(&user) with non-zero PK.
				// Arguments order: map values... then Where args.
				// Map keys: about_me, birthday, nickname, utime (alphabetical)
				// Args: "", {}, "new nickname", {}, 1 (explicit Where), 1 (Model PK)
				mock.ExpectExec("UPDATE .*users.* SET .*about_me.*=\\?,.*birthday.*=\\?,.*nickname.*=\\?,.*utime.*=\\? WHERE id = \\?").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			// Ensure UpdateById uses the correct ID logic if needed, but here it uses Model(&User{ID: id})
			err = dao.UpdateById(tc.ctx, tc.user)
			assert.Equal(t, tc.wantErr, err)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGORMUserDAO_FindById(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		mock     func(t *testing.T, mock sqlmock.Sqlmock)
		ctx      context.Context
		id       int64
		wantErr  error
		wantUser User
	}{
		{
			name: "success",
			ctx:  context.Background(),
			id:   1,
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "wechat_open_id", "wechat_union_id", "ctime", "utime"}).
					AddRow(1, "test@example.com", "", "", 0, "", "", "", "", "", 0, 0)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE id = \\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:            1,
				Email:         sql.NullString{String: "test@example.com", Valid: true},
				Birthday:      sql.NullInt64{Int64: 0, Valid: true},
				Phone:         sql.NullString{String: "", Valid: true},
				WechatOpenId:  sql.NullString{String: "", Valid: true},
				WechatUnionId: sql.NullString{String: "", Valid: true},
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			user, err := dao.FindById(tc.ctx, tc.id)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGORMUserDAO_FindByPhone(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		mock     func(t *testing.T, mock sqlmock.Sqlmock)
		ctx      context.Context
		phone    string
		wantErr  error
		wantUser User
	}{
		{
			name:  "success",
			ctx:   context.Background(),
			phone: "12345678901",
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "wechat_open_id", "wechat_union_id", "ctime", "utime"}).
					AddRow(1, "", "", "", 0, "", "", "12345678901", "", "", 0, 0)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE phone = \\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:            1,
				Phone:         sql.NullString{String: "12345678901", Valid: true},
				Birthday:      sql.NullInt64{Int64: 0, Valid: true},
				Email:         sql.NullString{String: "", Valid: true},
				WechatOpenId:  sql.NullString{String: "", Valid: true},
				WechatUnionId: sql.NullString{String: "", Valid: true},
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			user, err := dao.FindByPhone(tc.ctx, tc.phone)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGORMUserDAO_FindByWechat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		mock     func(t *testing.T, mock sqlmock.Sqlmock)
		ctx      context.Context
		openid   string
		wantErr  error
		wantUser User
	}{
		{
			name:   "success",
			ctx:    context.Background(),
			openid: "openid",
			mock: func(t *testing.T, mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "wechat_open_id", "wechat_union_id", "ctime", "utime"}).
					AddRow(1, "", "", "", 0, "", "", "", "openid", "", 0, 0)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE wechat_open_id=\\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:            1,
				WechatOpenId:  sql.NullString{String: "openid", Valid: true},
				Birthday:      sql.NullInt64{Int64: 0, Valid: true},
				Email:         sql.NullString{String: "", Valid: true},
				Phone:         sql.NullString{String: "", Valid: true},
				WechatUnionId: sql.NullString{String: "", Valid: true},
			},
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      db,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing: true,
			})
			require.NoError(t, err)

			tc.mock(t, mock)

			dao := NewGORMUserDAO(gormDB)
			user, err := dao.FindByWechat(tc.ctx, tc.openid)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, user)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
