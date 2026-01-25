package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	// Ensure the binary path is correct relative to where you run the command
	transport := &mcp.CommandTransport{Command: exec.Command("./bin/weather_server")}
	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("error while connecting to mcp client: %w", err)
	}
	defer session.Close()

	var serverTools []llms.Tool
	mcpTools, err := session.ListTools(ctx, nil)
	if err != nil {
		return err
	}

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

	llm, err := googleai.New(ctx,
		googleai.WithAPIKey(GeminiAPIKey),
		googleai.WithDefaultModel("gemini-2.5-flash-lite"),
	)
	if err != nil {
		return err
	}

	contextHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant using tools for weather."),
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("--- Weather Agent Ready (Go) ---")

	for {
		fmt.Printf("\nUser: ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		if strings.ToLower(input) == "exit" {
			fmt.Println("Bye Bye!")
			return nil
		}

		// FIX: Use '=' not ':=' so we don't shadow the contextHistory variable
		contextHistory = append(contextHistory, llms.TextParts(llms.ChatMessageTypeHuman, input))

		for {
			resp, err := llm.GenerateContent(ctx, contextHistory, llms.WithTools(serverTools), llms.WithMaxTokens(1000))
			if err != nil {
				// If 400 persists, it might be a safety block or an empty message array
				fmt.Printf("LLM Error: %v\n", err)
				break
			}

			choice := resp.Choices[0]

			if len(choice.ToolCalls) > 0 {
				toolCall := choice.ToolCalls[0]

				// 1. Add AI's intent to history
				contextHistory = append(contextHistory, llms.MessageContent{
					Role: llms.ChatMessageTypeAI,
					Parts: []llms.ContentPart{
						llms.ToolCall{
							ID: toolCall.ID, Type: "function",
							FunctionCall: &llms.FunctionCall{
								Name: toolCall.FunctionCall.Name, Arguments: toolCall.FunctionCall.Arguments,
							},
						},
					},
				})

				// 2. Parse arguments (Must use &args for Unmarshal)
				args := make(map[string]any)
				if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
					fmt.Printf("Unmarshal Error: %v\n", err)
					break
				}

				// 3. Execute MCP Call
				fmt.Printf("[Agent uses tool: %s]\n", toolCall.FunctionCall.Name)
				res, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name:      toolCall.FunctionCall.Name,
					Arguments: args,
				})
				if err != nil {
					fmt.Printf("MCP Tool Error: %v\n", err)
					break
				}

				var txt strings.Builder
				for _, c := range res.Content {
					if t, ok := c.(*mcp.TextContent); ok {
						txt.WriteString(t.Text)
					}
				}

				// 4. Add Tool Response to history
				contextHistory = append(contextHistory, llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: toolCall.ID,
							Name:       toolCall.FunctionCall.Name,
							Content:    txt.String(),
						},
					},
				})

				continue // Return to AI to process the result
			}

			// Final Response
			fmt.Printf("\nAI: %s\n", choice.Content)
			contextHistory = append(contextHistory, llms.TextParts(llms.ChatMessageTypeAI, choice.Content))
			break
		}
	}
	return nil
}
