package main

import (
	"context"
	"log"

	"github.com/ollama/ollama/api"
)

// runInference 调用 Ollama 进行推理
func (a *Agent) runInference(ctx context.Context, conversation []api.Message, tools []api.Tool) (api.Message, error) {
	if a.verbose {
		log.Printf("Making API call to Ollama with model: %s and %d tools", a.model, len(tools))
	}

	a.InputLock()
	defer a.InputUnLock()

	// 禁用流式传输以简化响应处理
	stream := false
	req := &api.ChatRequest{
		Model:    a.model,
		Messages: conversation,
		Tools:    tools,
		Stream:   &stream,
	}

	var responseMessage api.Message

	// 响应回调函数
	respFunc := func(resp api.ChatResponse) error {
		responseMessage = resp.Message
		return nil
	}

	// 执行聊天请求
	err := a.ollamaClient.Chat(ctx, req, respFunc)
	if err != nil {
		if a.verbose {
			log.Printf("API call failed: %v", err)
		}
		return api.Message{}, err
	}

	if a.verbose {
		log.Printf("API call successful, response received")
	}

	return responseMessage, nil
}
