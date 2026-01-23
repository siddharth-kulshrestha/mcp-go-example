package client

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

var GeminiAPIKey string

func init() {
	GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
}

func StartClient() error {

	ctx := context.Background()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "weather_client", Version: "0.0.0"}, nil)
	transport := &mcp.CommandTransport{Command: exec.Command("./weather_server")}
	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}
	defer session.Close()

	var serverTools []llms.Tool
	mcpTools, err := session.ListTools(ctx, nil)
	if err != nil {
		return err
	}
	fmt.Println(mcpTools)

	for _, tool := range mcpTools.Tools {
		serverTools = append(serverTools, llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	// Setup Gemini LLM
	llm, err := googleai.New(ctx, googleai.WithAPIKey(GeminiAPIKey))
	if err != nil {
		return err
	}

	// Agent state
	initialContext := []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant using tools for weather.")}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("--- Weather Agent Ready (Go) ---")

	for {
		fmt.Printf("\nUser: ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()

	}

}
