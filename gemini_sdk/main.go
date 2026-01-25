package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")

	// 1. Initialize the official unified client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	model := "gemini-3-flash-preview"
	prompt := "Explain how AI works in a few words"

	// 2. Set a ThinkingBudget to ensure reasoning happens but yields an answer
	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  genai.Ptr(int32(1024)), // Allow up to 1024 tokens of "thought"
		},
	}

	resp, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), config)
	if err != nil {
		log.Fatal(err)
	}

	// 3. The .Text() helper automatically finds the text part even if thoughts are present
	fmt.Printf("AI Response: %s\n", resp.Text())

	// You can also inspect the thought usage
	if resp.UsageMetadata != nil {
		fmt.Printf("(Used %d tokens for internal reasoning)\n", resp.UsageMetadata.ThoughtsTokenCount)
	}
}
