package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

type PMAgent struct {
	LLM     *googleai.GoogleAI
	History []llms.MessageContent
}

func NewPMAgent(ctx context.Context) (*PMAgent, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	llm, err := googleai.New(ctx, googleai.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	history := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are an autonomous Product Manager. Transform ideas into structured User Stories and tasks."),
	}

	return &PMAgent{
		LLM:     llm,
		History: history,
	}, nil
}

func (a *PMAgent) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	a.History = append(a.History, llms.TextParts(llms.ChatMessageTypeGeneric, prompt))

	resp, err := a.LLM.GenerateContent(ctx, a.History, llms.WithModel("gemini-2.0-flash"))
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from agent")
	}

	responseText := resp.Choices[0].Content
	a.History = append(a.History, llms.TextParts(llms.ChatMessageTypeAI, responseText))

	return responseText, nil
}
