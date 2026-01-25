package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

var GeminiAPIKey string

var ErrShouldContinue = fmt.Errorf("err not harmful")

func init() {
	GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
}

// HandlePrompts handles the prompts coming from user as part of input
func HandlePrompts(ctx context.Context, sess *mcp.ClientSession, input string) (string, error) {

	strs, err := shlex.Split(input)
	if err != nil {
		return "", fmt.Errorf("error while splitting the user input, err: %w", err)
	}

	if len(strs) < 2 {
		fmt.Println("\nUsage: /prompt <prompt_name> \"arg1\" \"arg2\" ...")
		return "", fmt.Errorf("not a valid prompt %w", ErrShouldContinue)
	}

	promptName := strs[1]
	promptArgs := strs[2:]
	res, err := sess.ListPrompts(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("Got err, while listing prompts: %w", err)
	}

	var targetPrompt *mcp.Prompt
	for _, p := range res.Prompts {
		if p.Name == promptName {
			targetPrompt = p
			break
		}
	}

	if targetPrompt == nil {
		return "", fmt.Errorf("prompt by name %s does not exists on server %w", promptName, ErrShouldContinue)
	}

	if len(promptArgs) < len(targetPrompt.Arguments) {
		return "", fmt.Errorf("length of arguments for prompt %s is not enough to invoke the prompt %w", promptName, ErrShouldContinue)
	}

	var argMap map[string]string

	for i, arg := range targetPrompt.Arguments {
		argMap[arg.Name] = promptArgs[i]
	}

	getPromptRes, err := sess.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      promptName,
		Arguments: argMap,
	})
	if err != nil {
		return "", fmt.Errorf("Error while, getting prompt err: %w, %w", err, ErrShouldContinue)
	}

	b, err := json.Marshal(getPromptRes.Messages[0].Content)
	if err != nil {
		return "", fmt.Errorf("Error while, marshalling prompt err: %w, %w", err, ErrShouldContinue)
	}

	fmt.Println("\n--- Prompt loaded successfully. Preparing to execute... ---")
	return string(b), nil
}

func ListPrompts(ctx context.Context, sess *mcp.ClientSession) error {
	res, err := sess.ListPrompts(ctx, nil)
	if err != nil {
		return fmt.Errorf("Got err, while listing prompts: %w", err)
	}
	fmt.Println("--------Available Prompts and their arguments-------------")
	for _, prompt := range res.Prompts {
		fmt.Printf("Prompt: %s \n", prompt.Name)
		if len(prompt.Arguments) > 0 {
			var argList []string
			for _, arg := range prompt.Arguments {
				argList = append(argList, arg.Name)
			}
			fmt.Printf("Arguments for prompt: %s : %s \n", prompt.Name, strings.Join(argList, ", "))
		} else {
			fmt.Printf("Arguments for prompt %s: None \n", prompt.Name)
		}
	}

	fmt.Println("\nUsage: /prompt <prompt_name> \"arg1\" \"arg2\" ...")
	fmt.Println("-----------------------------------------------------")
	return nil
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
		// CLEANUP: Gemini is strict. Let's ensure Parameters isn't nil
		params := tool.InputSchema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}

		serverTools = append(serverTools, llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
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

	contextHistory := []llms.MessageContent{}
	// contextHistory := []llms.MessageContent{
	// 	llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant using tools for weather."),
	// }

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("--- Weather Agent Ready (Go) ---")
	fmt.Println("Type a question, or use one of the following commands:")
	fmt.Println("  /prompts                           - to list available prompts")
	fmt.Println("  /prompt <prompt_name> \"args\"...  - to run a specific prompt")

	for {
		fmt.Printf("\nUser: ")
		if !scanner.Scan() {
			break
		}
		ip := scanner.Text()
		if strings.ToLower(ip) == "exit" {
			fmt.Println("Bye Bye!")
			return nil
		}

		input := ip

		// Check if user has asked to list down prompts
		if strings.ToLower(ip) == "/prompts" {
			err := ListPrompts(ctx, session)
			if err != nil {
				fmt.Println("error while listing the prompts from the MCP Server, err: ", err)
				continue
			}
		} else if strings.HasPrefix(ip, "/prompt") {
			prompt, err := HandlePrompts(ctx, session, ip)
			if err != nil {
				fmt.Printf("Error occured while handling the prompts, err: %v\n", err)
				continue
			}
			if len(prompt) > 0 {
				input = ip
			}
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
