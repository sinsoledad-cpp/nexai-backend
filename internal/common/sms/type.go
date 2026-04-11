package sms

import "context"

//go:generate mockgen -source=./type.go -package=mocks -destination=./mocks/sms_mock.go Service
type Service interface {
	Send(ctx context.Context, tplId string, args []string, numbers ...string) error
}
