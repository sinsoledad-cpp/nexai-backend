package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type LLMConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Region   string
}

func NewChatModel(cfg LLMConfig) (model.BaseChatModel, error) {
	switch cfg.Provider {
	case "ark":
		return newArkChatModel(cfg)
	default:
		return newOpenAIChatModel(cfg)
	}
}

func newArkChatModel(cfg LLMConfig) (model.BaseChatModel, error) {
	arkCfg := &ark.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.Region != "" {
		arkCfg.Region = cfg.Region
	}
	m, err := ark.NewChatModel(context.Background(), arkCfg)
	if err != nil {
		return nil, fmt.Errorf("创建Ark ChatModel失败: %w", err)
	}
	return m, nil
}

func newOpenAIChatModel(cfg LLMConfig) (model.BaseChatModel, error) {
	m, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("创建OpenAI ChatModel失败: %w", err)
	}
	return m, nil
}

type ChatModelAdapter struct {
	model model.BaseChatModel
}

func NewChatModelAdapter(m model.BaseChatModel) *ChatModelAdapter {
	return &ChatModelAdapter{model: m}
}

func (a *ChatModelAdapter) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return a.model.Generate(ctx, input, opts...)
}
