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
	contextHistory := []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant using tools for weather.")}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("--- Weather Agent Ready (Go) ---")

	for {
		fmt.Printf("\nUser: ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		// Exit condition
		if strings.ToLower(input) == "exit" {
			fmt.Println("Bye Bye!")
			return nil
		}
		contextHistory := append(contextHistory, llms.TextParts(llms.ChatMessageTypeHuman, input))

		// Tool Calling loop:
		// AI -> Call Tool -> AI -> Call Tool ..... -> AI -> Call Tool -> AI Response
		for {
			resp, err := llm.GenerateContent(ctx, contextHistory, llms.WithTools(serverTools))
			if err != nil {
				fmt.Printf("Something went wrong, error occured: %v", err)
				return err
			}

			choice := resp.Choices[0]

			// Case 1: If LLM wants to call to a tool
			if len(choice.ToolCalls) > 0 {
				toolCall := choice.ToolCalls[0]

				// Logging AI's intent to call the tool
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

				// Parse arguments
				args := map[string]any{}
				err = json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), args)
				if err != nil {
					fmt.Printf("Something went wrong, error occured: %v", err)
					return err
				}

				// Execute MCP Call
				fmt.Printf("[Agent uses tool: %s with args: %v]\n", toolCall.FunctionCall.Name, args)
				res, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name:      toolCall.FunctionCall.Name,
					Arguments: args,
				})
				if err != nil {
					fmt.Printf("Something went wrong, error occured: %v", err)
					return err
				}

				// Collecting tool results
				var txt strings.Builder
				for _, c := range res.Content {
					if t, ok := c.(*mcp.TextContent); ok {
						txt.WriteString(t.Text)
					}
				}

				// Feed back the result to contextHistory
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

				// Feed back
				continue
			}

			// Case 2: LLM gives back the answer
			fmt.Printf("\n AI: %s \n", choice.Content)
			contextHistory = append(contextHistory, llms.TextParts(llms.ChatMessageTypeAI, choice.Content))
			break
		}

	}
	return nil

}
