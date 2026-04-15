package ioc

import (
	"nexai-backend/internal/resume/service"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/spf13/viper"
)

func InitChatModel() model.BaseChatModel {
	cfg := service.LLMConfig{
		Provider: envOrViper("LLM_PROVIDER", "llm.provider"),
		BaseURL:  envOrViper("LLM_BASE_URL", "llm.baseURL"),
		APIKey:   envOrViper("LLM_API_KEY", "llm.apiKey"),
		Model:    envOrViper("LLM_MODEL", "llm.model"),
		Region:   envOrViper("LLM_REGION", "llm.region"),
	}

	if cfg.Provider == "ark" {
		if cfg.Region == "" {
			cfg.Region = "cn-beijing"
		}
	} else {
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
		if cfg.Model == "" {
			cfg.Model = "gpt-4o-mini"
		}
	}

	//fmt.Printf("cfg:= #v", cfg)

	m, err := service.NewChatModel(cfg)
	if err != nil {
		panic(err)
	}
	return m
}

func envOrViper(envKey, viperKey string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	return viper.GetString(viperKey)
}
