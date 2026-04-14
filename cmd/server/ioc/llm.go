package ioc

import (
	"nexai-backend/internal/resume/service"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/spf13/viper"
)

func InitChatModel() model.BaseChatModel {
	cfg := service.LLMConfig{
		BaseURL: viper.GetString("llm.baseURL"),
		APIKey:  viper.GetString("llm.apiKey"),
		Model:   viper.GetString("llm.model"),
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("LLM_BASE_URL")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("LLM_API_KEY")
	}
	if cfg.Model == "" {
		cfg.Model = os.Getenv("LLM_MODEL")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}

	m, err := service.NewChatModel(cfg)
	if err != nil {
		panic(err)
	}
	return m
}
