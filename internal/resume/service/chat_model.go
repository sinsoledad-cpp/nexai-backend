package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewChatModel(cfg LLMConfig) (model.BaseChatModel, error) {
	m, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("创建ChatModel失败: %w", err)
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
