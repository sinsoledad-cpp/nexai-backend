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
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "ctime", "utime"}).
					AddRow(1, "test@example.com", "password", "nickname", 0, "avatar", "about", "12345678901", 123, 123)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE email=\\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:       1,
				Email:    sql.NullString{String: "test@example.com", Valid: true},
				Password: "password",
				Nickname: "nickname",
				Birthday: sql.NullInt64{Int64: 0, Valid: true},
				Avatar:   "avatar",
				AboutMe:  "about",
				Phone:    sql.NullString{String: "12345678901", Valid: true},
				Ctime:    123,
				Utime:    123,
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
				// 使用更简单的正则表达式来匹配 SQL 语句
				mock.ExpectExec("UPDATE.*users.*SET.*nickname.*utime.*WHERE.*id.*").
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
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "ctime", "utime"}).
					AddRow(1, "test@example.com", "", "", 0, "", "", "", 0, 0)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE id = \\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:       1,
				Email:    sql.NullString{String: "test@example.com", Valid: true},
				Birthday: sql.NullInt64{Int64: 0, Valid: true},
				Phone:    sql.NullString{String: "", Valid: true},
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
				rows := sqlmock.NewRows([]string{"id", "email", "password", "nickname", "birthday", "avatar", "about_me", "phone", "ctime", "utime"}).
					AddRow(1, "", "", "", 0, "", "", "12345678901", 0, 0)
				mock.ExpectQuery("SELECT .* FROM .*users.* WHERE phone = \\?.*").
					WillReturnRows(rows)
			},
			wantUser: User{
				ID:       1,
				Phone:    sql.NullString{String: "12345678901", Valid: true},
				Birthday: sql.NullInt64{Int64: 0, Valid: true},
				Email:    sql.NullString{String: "", Valid: true},
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
