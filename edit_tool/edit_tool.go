package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

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

	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, BashToolDefinition, EditFileDefinition}
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
									if a.verbose {
										log.Printf("Tool Error: %v", toolError)
									}
									return err
								} else {
									if a.verbose {
										log.Printf("Tool %s executed successfully", tool.Name)
									}
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

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List all files and directories at a given relative path. If no path is provided, list files in the current working directory.",
	InputSchema: api.ToolFunctionParameters{
		Type:     "object",
		Required: []string{"path"},
		Properties: map[string]api.ToolProperty{
			"path": {
				Type:        api.PropertyType{"string"},
				Description: "Optional relative path to list files from. Defaults to current directory if not provided.",
			},
		},
	},
	Function: ListFiles,
}

type ListFilesInput struct {
	Path string `json:"path,omitempty"`
}

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal list_files input: %w", err)
	}
	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	log.Printf("ListFiles path: %s", dir)

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	log.Printf("Successfully listed %d files in %s", len(files), dir)

	result, err := json.Marshal(files)
	if err != nil {
		return "", fmt.Errorf("failed to marshal list of files: %w", err)
	}
	return string(result), nil
}

var BashToolDefinition = ToolDefinition{
	Name:        "bash",
	Description: "Execute a bash command and return the output. Use this tool when you need to run a bash command in the working directory.",
	InputSchema: api.ToolFunctionParameters{
		Type:     "object",
		Required: []string{"command"},
		Properties: map[string]api.ToolProperty{
			"command": {
				Type:        api.PropertyType{"string"},
				Description: "The bash command to execute.",
			},
		},
	},
	Function: Bash,
}

type BashInput struct {
	Command string `json:"command"`
}

func Bash(input json.RawMessage) (string, error) {
	bashInput := BashInput{}
	if err := json.Unmarshal(input, &bashInput); err != nil {
		return "", fmt.Errorf("failed to unmarshal bash input: %w", err)
	}
	log.Printf("Bash command: %s", bashInput.Command)

	cmd := exec.Command("bash", "-c", bashInput.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute bash command: %w", err)
	}
	log.Printf("Bash command successfully executed: %s, output length: %d", bashInput.Command, len(output))
	return strings.TrimSpace(string(output)), nil
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a text file.

Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

If the file specified with path doesn't exist, it will be created.
`,
	InputSchema: api.ToolFunctionParameters{
		Type:     "object",
		Required: []string{"path", "old_str", "new_str"},
		Properties: map[string]api.ToolProperty{
			"path": {
				Type:        api.PropertyType{"string"},
				Description: "The path to the file",
			},
			"old_str": {
				Type:        api.PropertyType{"string"},
				Description: "Text to search for - must match exactly and must only have one match exactly",
			},
			"new_str": {
				Type:        api.PropertyType{"string"},
				Description: "Text to replace old_str with",
			},
		},
	},
	Function: EditFile,
}

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for - must match exactly and must only have one match exactly"`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		log.Printf("EditFile failed: invalid input parameters")
		return "", fmt.Errorf("invalid input parameters")
	}

	log.Printf("Editing file: %s (replacing %d chars with %d chars)", editFileInput.Path, len(editFileInput.OldStr), len(editFileInput.NewStr))
	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldStr == "" {
			log.Printf("File does not exist, creating new file: %s", editFileInput.Path)
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		log.Printf("Failed to read file %s: %v", editFileInput.Path, err)
		return "", err
	}

	oldContent := string(content)

	// Special case: if old_str is empty, we're appending to the file
	var newContent string
	if editFileInput.OldStr == "" {
		newContent = oldContent + editFileInput.NewStr
	} else {
		// Count occurrences first to ensure we have exactly one match
		count := strings.Count(oldContent, editFileInput.OldStr)
		if count == 0 {
			log.Printf("EditFile failed: old_str not found in file %s", editFileInput.Path)
			return "", fmt.Errorf("old_str not found in file")
		}
		if count > 1 {
			log.Printf("EditFile failed: old_str found %d times in file %s, must be unique", count, editFileInput.Path)
			return "", fmt.Errorf("old_str found %d times in file, must be unique", count)
		}

		newContent = strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, 1)
	}

	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		log.Printf("Failed to write file %s: %v", editFileInput.Path, err)
		return "", err
	}

	log.Printf("Successfully edited file %s", editFileInput.Path)
	return "OK", nil
}

func createNewFile(filePath, content string) (string, error) {
	log.Printf("Creating new file: %s (%d bytes)", filePath, len(content))
	dir := path.Dir(filePath)
	if dir != "." {
		log.Printf("Creating directory: %s", dir)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.Printf("Failed to create directory %s: %v", dir, err)
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		log.Printf("Failed to create file %s: %v", filePath, err)
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	log.Printf("Successfully created file %s", filePath)
	return fmt.Sprintf("Successfully created file %s", filePath), nil
}
