package bootstrap

import "nexai-backend/pkg/validate"

func InitValidate() {
	if err := validate.InitTrans("zh"); err != nil {
		panic(err)
	}
}
