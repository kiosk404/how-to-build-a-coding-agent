package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ollama/ollama/api"
)

func (a *Agent) runInferenceStreaming(ctx context.Context, conversation []api.Message, tools []api.Tool) (api.Message, error) {
	if a.verbose {
		log.Printf("Making streaming request with model: %v and %d tools", a.model, len(tools))
	}

	// 启用流式传输
	stream := true
	req := &api.ChatRequest{
		Model:    a.model,
		Stream:   &stream,
		Messages: conversation,
		Tools:    tools,
	}

	var finalMessage api.Message
	var contentBuilder string

	// 流式响应
	respFunc := func(resp api.ChatResponse) error {
		// 实时传输文本内容
		if resp.Message.Content != "" {
			fmt.Print(resp.Message.Content)
			contentBuilder += resp.Message.Content
		}

		if resp.Done {
			finalMessage = resp.Message
			finalMessage.Content = contentBuilder
			fmt.Print("\r\n")
		}

		// 收集工具调用
		if len(resp.Message.ToolCalls) > 0 {
			finalMessage.ToolCalls = append(finalMessage.ToolCalls, resp.Message.ToolCalls...)
		}

		return nil
	}

	// 发送流式请求
	if err := a.ollamaClient.Chat(ctx, req, respFunc); err != nil {
		if a.verbose {
			log.Printf("Chat streaming error: %v", err)
		}
		return api.Message{}, fmt.Errorf("chat streaming error: %w", err)
	}

	if a.verbose {
		log.Printf("Streaming API call successful, response received")
	}

	return finalMessage, nil
}
