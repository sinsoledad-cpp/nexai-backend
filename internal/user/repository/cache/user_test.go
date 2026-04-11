package cache

import (
	"context"
	"errors"
	"nexai-backend/internal/domain"
	"testing"
	"time"

	json "github.com/json-iterator/go"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisUserCache_Get(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		mock     func(mock redismock.ClientMock)
		id       int64
		wantUser domain.User
		wantErr  error
	}{
		{
			name: "success",
			id:   1,
			mock: func(mock redismock.ClientMock) {
				u := domain.User{
					ID:       1,
					Email:    "test@example.com",
					Nickname: "test",
				}
				data, _ := json.Marshal(u)
				mock.ExpectGet("user:info:1").SetVal(string(data))
			},
			wantUser: domain.User{
				ID:       1,
				Email:    "test@example.com",
				Nickname: "test",
			},
			wantErr: nil,
		},
		{
			name: "cache miss",
			id:   1,
			mock: func(mock redismock.ClientMock) {
				mock.ExpectGet("user:info:1").RedisNil()
			},
			wantUser: domain.User{},
			wantErr:  ErrKeyNotExist,
		},
		{
			name: "redis error",
			id:   1,
			mock: func(mock redismock.ClientMock) {
				mock.ExpectGet("user:info:1").SetErr(errors.New("redis error"))
			},
			wantUser: domain.User{},
			wantErr:  errors.New("redis error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, mock := redismock.NewClientMock()
			tc.mock(mock)
			c := NewRedisUserCache(db)
			u, err := c.Get(context.Background(), tc.id)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.wantUser, u)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRedisUserCache_Set(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(mock redismock.ClientMock)
		user    domain.User
		wantErr error
	}{
		{
			name: "success",
			user: domain.User{
				ID:       1,
				Email:    "test@example.com",
				Nickname: "test",
			},
			mock: func(mock redismock.ClientMock) {
				u := domain.User{
					ID:       1,
					Email:    "test@example.com",
					Nickname: "test",
				}
				data, _ := json.Marshal(u)
				mock.ExpectSet("user:info:1", data, time.Minute*15).SetVal("OK")
			},
			wantErr: nil,
		},
		{
			name: "redis error",
			user: domain.User{
				ID: 1,
			},
			mock: func(mock redismock.ClientMock) {
				u := domain.User{
					ID: 1,
				}
				data, _ := json.Marshal(u)
				mock.ExpectSet("user:info:1", data, time.Minute*15).SetErr(errors.New("redis error"))
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
			c := NewRedisUserCache(db)
			err := c.Set(context.Background(), tc.user)
			assert.Equal(t, tc.wantErr, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRedisUserCache_Delete(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		mock    func(mock redismock.ClientMock)
		id      int64
		wantErr error
	}{
		{
			name: "success",
			id:   1,
			mock: func(mock redismock.ClientMock) {
				mock.ExpectDel("user:info:1").SetVal(1)
			},
			wantErr: nil,
		},
		{
			name: "redis error",
			id:   1,
			mock: func(mock redismock.ClientMock) {
				mock.ExpectDel("user:info:1").SetErr(errors.New("redis error"))
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
			c := NewRedisUserCache(db)
			err := c.Delete(context.Background(), tc.id)
			assert.Equal(t, tc.wantErr, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
