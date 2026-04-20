package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"

	"github.com/bryanbaek/mission/internal/controlplane/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: preflight-helper migrations-status --database-url=<url>")
	}

	switch args[0] {
	case "migrations-status":
		return runMigrationsStatus(args[1:])
	case "anthropic-ping":
		return runAnthropicPing(args[1:])
	case "openai-ping":
		return runOpenAIPing(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runMigrationsStatus(args []string) error {
	fs := flag.NewFlagSet("migrations-status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var databaseURL string
	fs.StringVar(&databaseURL, "database-url", "", "Postgres database URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if databaseURL == "" {
		return fmt.Errorf("--database-url is required")
	}

	status, err := db.MigrationState(databaseURL)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(status)
}

func runAnthropicPing(args []string) error {
	fs := flag.NewFlagSet("anthropic-ping", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var apiKey string
	var model string
	fs.StringVar(&apiKey, "api-key", "", "Anthropic API key")
	fs.StringVar(&model, "model", "", "Anthropic model")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if apiKey == "" {
		return fmt.Errorf("--api-key is required")
	}
	if model == "" {
		return fmt.Errorf("--model is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := anthropic.NewClient(anthropicoption.WithAPIKey(apiKey))
	_, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ping")),
		},
	})
	return err
}

func runOpenAIPing(args []string) error {
	fs := flag.NewFlagSet("openai-ping", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var apiKey string
	var model string
	fs.StringVar(&apiKey, "api-key", "", "OpenAI API key")
	fs.StringVar(&model, "model", "", "OpenAI model")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if apiKey == "" {
		return fmt.Errorf("--api-key is required")
	}
	if model == "" {
		return fmt.Errorf("--model is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := openai.NewClient(openaioption.WithAPIKey(apiKey))
	_, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model:           openai.ResponsesModel(model),
		MaxOutputTokens: openai.Int(1),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("ping"),
		},
	})
	return err
}
