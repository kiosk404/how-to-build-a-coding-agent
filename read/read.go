package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ollama/ollama/api"
)

type Agent struct {
	client  *api.Client
	model   string
	tools   []ToolDefinition
	verbose bool
}

func NewAgent(client *api.Client, model string, tools []ToolDefinition, verbose bool) *Agent {
	return &Agent{
		client:  client,
		model:   model,
		tools:   tools,
		verbose: verbose,
	}
}

func main() {
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	model := flag.String("model", "llama3.1", "the model to use for the agent")
	flag.Parse()

	if *verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("verbose logging enabled, model: %s", *model)
	} else {
		log.SetOutput(os.Stdout)
		log.SetFlags(0)
		log.Printf("")
	}

	// Initialize Ollama client from environment (OLLAMA HOST)
	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatalf("failed to initialize Ollama client: %v", err)
	}

	tools := []ToolDefinition{ReadFileDefinition}
	if *verbose {
		log.Printf("starting conversation with model: %s Initializing %d tools", *model, len(tools))
	}
	agent := NewAgent(client, *model, tools, *verbose)
	if err := agent.Run(context.Background()); err != nil {
		log.Fatalf("error running agent: %v", err)
	}
}

func (a *Agent) Run(ctx context.Context) error {
	var conversation []api.Message
	if a.verbose {
		log.Printf("starting conversation with model: %s", a.model)
	}
	fmt.Println("Chat with Ollama (type 'exit' to quit)")

	for {
		var userInput string
		prompt := &survey.Input{
			Message: "\033[32mYou:\033[0m",
		}
		err := survey.AskOne(prompt, &userInput)
		if err != nil {
			if a.verbose {
				log.Printf("error asking user input: %v", err)
			}
			break
		}

		// skip empty input
		if userInput == "" {
			continue
		}

		userMessage := api.Message{Role: "user", Content: userInput}
		conversation = append(conversation, userMessage)

		if a.verbose {
			log.Printf("Sending message to ollama, conversation length: %d", len(conversation))
		}

		message, err := a.runInference(ctx, conversation)
		if err != nil {
			if a.verbose {
				log.Printf("error running inference: %v", err)
			}
			fmt.Printf("run failed: %v", err.Error())
			break
		}
		conversation = append(conversation, message)

		// Keep processing until Ollama stops using tools
		for {
			// Display text content
			if message.Content != "" {
				fmt.Println("\u001b[34mOllama:\u001b[0m", message.Content)
			}

			// Check for tool calls
			var hasToolUse bool
			if len(message.ToolCalls) > 0 {
				hasToolUse = true
				if a.verbose {
					log.Printf("Proccessing %d tool calls from Ollama", len(message.ToolCalls))
				}

				// Process each tool call
				for _, toolCall := range message.ToolCalls {
					argsJSON, _ := json.Marshal(toolCall.Function.Arguments)
					if a.verbose {
						log.Printf("Tool use detected: %s, arguments: %s", toolCall.Function.Name, string(argsJSON))
					}
					fmt.Printf("\u001b[33mTool Input:\u001b[0m %s\n", string(argsJSON))

					// Find and execute the tool
					var toolResult string
					var toolError error
					var toolFound bool
					for _, tool := range a.tools {
						if tool.Name == toolCall.Function.Name {
							if tool.Name == toolCall.Function.Name {
								if a.verbose {
									log.Printf("Executing tool: %s", tool.Name)
								}
								//Convert arguments to JSON for tool function
								argsJSON, _ := json.Marshal(toolCall.Function.Arguments)
								toolResult, toolError = tool.Function(argsJSON)
								fmt.Printf("\u001b[32mTool Output:\u001b[0m %s\n", toolResult)
								if toolError != nil {
									log.Printf("Tool Error: %v", toolError)
								} else {
									log.Printf("Tool %s executed successfully", tool.Name)
								}
								toolFound = true
								break
							}
						}
					}

					if !toolFound {
						toolError = fmt.Errorf("tool '%s' not found", toolCall.Function.Name)
						fmt.Printf("\u001b[31mTool Error:\u001b[0m %v\n", toolError)
						toolResult = toolError.Error()
					}

					// Add tool result to conversation
					toolMessage := api.Message{
						Role:       "tool",
						Content:    toolResult,
						ToolName:   toolCall.Function.Name,
						ToolCallID: toolCall.ID,
					}
					conversation = append(conversation, toolMessage)
				}
			}
			// If no tool use, break the loop
			if !hasToolUse {
				break
			}

			if a.verbose {
				log.Printf("Sending message to ollama, conversation length: %d", len(conversation))
			}
			message, err = a.runInference(ctx, conversation)
			if err != nil {
				if a.verbose {
					log.Printf("error running inference: %v", err)
				}
				return err
			}
			conversation = append(conversation, message)
			if a.verbose {
				log.Printf("Received message from ollama, role=%s, content length: %v", message.Role, len(message.Content))
			}
		}
	}

	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []api.Message) (api.Message, error) {
	if a.verbose {
		log.Printf("Make API call to ollama, model: %s, conversation length: %d", a.model, len(conversation))
	}

	ollamaTools := []api.Tool{}
	for _, tool := range a.tools {
		ollamaTools = append(ollamaTools, api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	// Disable streaming for now
	stream := false
	req := &api.ChatRequest{
		Model:    a.model,
		Messages: conversation,
		Stream:   &stream,
		Tools:    ollamaTools,
	}

	var responseMessage api.Message

	// Response callback function
	respFunc := func(resp api.ChatResponse) error {
		responseMessage = resp.Message
		return nil
	}
	// Execute chat request
	err := a.client.Chat(ctx, req, respFunc)
	if err != nil {
		return api.Message{}, fmt.Errorf("failed to generate response: %w", err)
	}

	if a.verbose {
		log.Printf("API call successful, response role=%s, content length: %v", responseMessage.Role, len(responseMessage.Content))
	}

	return responseMessage, nil
}

type ToolDefinition struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	InputSchema api.ToolFunctionParameters `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this tool when you need to read the contents of a file in the working directory.",
	InputSchema: api.ToolFunctionParameters{
		Type:     "object",
		Required: []string{"path"},
		Properties: map[string]api.ToolProperty{
			"path": {
				Type:        api.PropertyType{"string"},
				Description: "The relative path of a file in the working directory.",
			},
		},
	},
	Function: ReadFile,
}

type ReadFileInput struct {
	Path string `json:"path"`
}

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	if err := json.Unmarshal(input, &readFileInput); err != nil {
		return "", fmt.Errorf("failed to unmarshal read_file input: %w", err)
	}
	log.Printf("ReadFile path: %s", readFileInput.Path)
	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	log.Printf("Successfully read file %s, content length: %d", readFileInput.Path, len(content))
	return string(content), nil
}
