package main

import (
	"context"
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
	verbose bool
}

func NewAgent(client *api.Client, model string, verbose bool) *Agent {
	return &Agent{
		client:  client,
		model:   model,
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

	agent := NewAgent(client, *model, *verbose)
	if *verbose {
		log.Printf("starting conversation with model: %s", *model)
	}
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

		reply, err := a.runInference(ctx, conversation)
		if err != nil {
			if a.verbose {
				log.Printf("error running inference: %v", err)
			}
			break
		}
		conversation = append(conversation, reply)

		fmt.Println("\u001b[34mOllama:\u001b[0m", reply.Content)
	}

	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []api.Message) (api.Message, error) {
	if a.verbose {
		log.Printf("Make API call to ollama, model: %s, conversation length: %d", a.model, len(conversation))
	}

	// Disable streaming for now
	stream := false
	req := &api.ChatRequest{
		Model:    a.model,
		Messages: conversation,
		Stream:   &stream,
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
