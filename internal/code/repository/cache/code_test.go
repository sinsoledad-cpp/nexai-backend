package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisCodeCache_Set(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(mock redismock.ClientMock)
		biz     string
		phone   string
		code    string
		wantErr error
	}{
		{
			name:  "success",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaSetCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(0))
			},
			wantErr: nil,
		},
		{
			name:  "send too many",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaSetCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(-1))
			},
			wantErr: ErrCodeSendTooMany,
		},
		{
			name:  "code exists no expire",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaSetCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(-2))
			},
			wantErr: errors.New("验证码存在，但是没有过期时间"),
		},
		{
			name:  "redis error",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaSetCode, []string{"phone_code:login:12345678901"}, "123456").SetErr(errors.New("redis error"))
			},
			wantErr: errors.New("redis error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, mock := redismock.NewClientMock()
			tc.mock(mock)
			c := NewRedisCodeCache(db)
			err := c.Set(context.Background(), tc.biz, tc.phone, tc.code)
			assert.Equal(t, tc.wantErr, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRedisCodeCache_Verify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(mock redismock.ClientMock)
		biz     string
		phone   string
		code    string
		wantOk  bool
		wantErr error
	}{
		{
			name:  "success",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaVerifyCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(0))
			},
			wantOk:  true,
			wantErr: nil,
		},
		{
			name:  "verify too many",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaVerifyCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(-1))
			},
			wantOk:  false,
			wantErr: ErrCodeVerifyTooMany,
		},
		{
			name:  "invalid code",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaVerifyCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(-2))
			},
			wantOk:  false,
			wantErr: nil,
		},
		{
			name:  "code expired",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaVerifyCode, []string{"phone_code:login:12345678901"}, "123456").SetVal(int64(-3))
			},
			wantOk:  false,
			wantErr: ErrCodeExpired,
		},
		{
			name:  "redis error",
			biz:   "login",
			phone: "12345678901",
			code:  "123456",
			mock: func(mock redismock.ClientMock) {
				mock.ExpectEval(luaVerifyCode, []string{"phone_code:login:12345678901"}, "123456").SetErr(errors.New("redis error"))
			},
			wantOk:  false,
			wantErr: errors.New("redis error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, mock := redismock.NewClientMock()
			tc.mock(mock)
			c := NewRedisCodeCache(db)
			ok, err := c.Verify(context.Background(), tc.biz, tc.phone, tc.code)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantOk, ok)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
