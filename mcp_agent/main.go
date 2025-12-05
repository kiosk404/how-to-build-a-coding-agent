package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/AlecAivazis/survey/v2"
	"github.com/kiosk404/how-to-build-a-coding-agent/pkg/mcp"
	"github.com/ollama/ollama/api"
)

func main() {
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	model := flag.String("model", "qwen3:1.7b", "Ollama model name")
	stream := flag.Bool("stream", false, "Enable streaming mode")
	configPath := flag.String("config", "", "MCP config file path (default: ./mcp_agent/mcp.json)")
	flag.Parse()

	if *verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Verbose logging enabled")
	} else {
		log.SetOutput(os.Stdout)
		log.SetFlags(0)
		log.SetPrefix("")
	}

	// 确定配置文件路径
	cfgPath := *configPath
	if cfgPath == "" {
		// 优先使用当前目录下的 map.json
		if _, err := os.Stat("map.json"); err == nil {
			cfgPath = "map.json"
		} else {
			// 打印当前工作目录
			cwd, _ := os.Getwd()
			mcpAgentDir := fmt.Sprintf("%s/mcp_agent", cwd)
			cfgPath = filepath.Join(mcpAgentDir, "map.json")
		}
	}

	// 加载 MCP 配置
	if *verbose {
		log.Printf("Loading MCP config from: %s", cfgPath)
	}
	config, err := mcp.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load MCP config: %v", err)
	}

	// 创建 MCP 客户端
	ctx := context.Background()
	mcpClient, err := mcp.NewClient(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create MCP client: %v", err)
	}
	defer mcpClient.Close()

	if *verbose {
		log.Println("MCP client initialized")
	}

	// 初始化 Ollama 客户端
	ollamaClient, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatalf("Failed to initialize Ollama client: %v", err)
	}
	if *verbose {
		log.Println("Ollama client initialized")
	}

	// 创建 Agent
	agent := NewAgent(ollamaClient, mcpClient, *model, *verbose, *stream)
	err = agent.Run(ctx)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

// Agent 是基于 MCP 的智能代理
type Agent struct {
	ollamaClient *api.Client
	mcpClient    *mcp.Client
	model        string
	verbose      bool
	stream       bool
	inputLock    sync.Mutex
	isProcessing bool
}

// NewAgent 创建一个新的 Agent 实例
func NewAgent(
	ollamaClient *api.Client,
	mcpClient *mcp.Client,
	model string,
	verbose bool,
	stream bool,
) *Agent {
	return &Agent{
		ollamaClient: ollamaClient,
		mcpClient:    mcpClient,
		model:        model,
		verbose:      verbose,
		stream:       stream,
	}
}

// Run 启动 Agent 的交互循环
func (a *Agent) Run(ctx context.Context) error {
	var conversation []api.Message

	// 获取 MCP 工具列表
	tools, err := a.mcpClient.GetTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to get MCP tools: %w", err)
	}

	if a.verbose {
		log.Printf("Loaded %d MCP tools", len(tools))
		for _, tool := range tools {
			log.Printf("  - %s: %s", tool.Function.Name, tool.Function.Description)
		}
	}

	fmt.Println("Chat with Ollama + MCP (use 'ctrl-c' to quit)")
	fmt.Printf("Available tools: %d\n", len(tools))

	for {
		var userInput string
		prompt := &survey.Input{
			Message: "\033[94mYou\033[0m:",
		}
		err := survey.AskOne(prompt, &userInput)
		if err != nil {
			if a.verbose {
				log.Printf("User input ended: %v", err)
			}
			break
		}

		// 跳过空消息
		if userInput == "" {
			if a.verbose {
				log.Println("Skipping empty message")
			}
			continue
		}

		if a.verbose {
			log.Printf("User input received: %q", userInput)
		}

		userMessage := api.Message{Role: "user", Content: userInput}
		conversation = append(conversation, userMessage)

		if a.verbose {
			log.Printf("Sending message to Ollama, conversation length: %d", len(conversation))
		}

		// 禁止用户输入
		//oldState, termErr := term.MakeRaw(int(os.Stdin.Fd()))
		//if termErr != nil && a.verbose {
		//	log.Printf("Warning: failed to set terminal raw mode: %v", termErr)
		//}

		var message api.Message
		if a.stream {
			fmt.Print("\u001b[93mOllama\u001b[0m:")
			if message, err = a.runInferenceStreaming(ctx, conversation, tools); err != nil {
				if a.verbose {
					log.Printf("Error during streaming inference: %v", err)
				}
				return err
			}
		} else {
			if message, err = a.runInference(ctx, conversation, tools); err != nil {
				if a.verbose {
					log.Printf("Error during inference: %v", err)
				}
				return err
			}
		}

		conversation = append(conversation, message)

		// 持续处理直到没有工具调用
		for {
			// 显示文本内容
			if !a.stream && message.Content != "" {
				fmt.Printf("\u001b[93mOllama\u001b[0m: %s\n", message.Content)
			}

			// 检查工具调用
			var hasToolUse bool
			if len(message.ToolCalls) > 0 {
				hasToolUse = true
				if a.verbose {
					log.Printf("Processing %d tool calls from Ollama", len(message.ToolCalls))
				}

				// 处理每个工具调用
				for _, toolCall := range message.ToolCalls {
					if a.verbose {
						argsJSON, _ := json.Marshal(toolCall.Function.Arguments)
						log.Printf("Tool use detected: %s with input: %s", toolCall.Function.Name, string(argsJSON))
					}
					argsJSON, _ := json.Marshal(toolCall.Function.Arguments)
					fmt.Printf("\u001b[96mtool\u001b[0m: %s(%s)\n", toolCall.Function.Name, string(argsJSON))

					// 通过 MCP 客户端调用工具
					result, err := a.mcpClient.CallTool(ctx, toolCall.Function.Name, toolCall.Function.Arguments)

					var toolResult string
					if err != nil {
						toolResult = fmt.Sprintf("Error: %v", err)
						fmt.Printf("\u001b[91merror\u001b[0m: %s\n", err.Error())
						if a.verbose {
							log.Printf("Tool execution failed: %v", err)
						}
					} else {
						// 将结果转换为字符串
						toolResult = formatToolResult(result)
						fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", truncateString(toolResult, 500))
						if a.verbose {
							log.Printf("Tool execution successful, result length: %d chars", len(toolResult))
						}
					}

					// 将工具结果添加到对话中
					conversation = append(conversation, api.Message{
						Role:     "tool",
						Content:  toolResult,
						ToolName: toolCall.Function.Name,
					})
				}
			}

			// 如果没有工具调用，结束循环
			if !hasToolUse {
				break
			}

			// 获取工具执行后的响应
			if a.verbose {
				log.Printf("Sending tool results back to Ollama")
			}
			message, err = a.runInference(ctx, conversation, tools)
			if err != nil {
				if a.verbose {
					log.Printf("Error during followup inference: %v", err)
				}
				return err
			}
			conversation = append(conversation, message)

			if a.verbose {
				log.Printf("Received followup response")
			}
		}

		// 恢复终端状态，允许用户输入
		//if oldState != nil {
		//	term.Restore(int(os.Stdin.Fd()), oldState)
		//}
	}

	if a.verbose {
		log.Println("Chat session ended")
	}
	return nil
}
