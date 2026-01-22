package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InputOutput struct {
	A      int
	B      int
	Result int
}

func Subtract(ctx context.Context, req *mcp.CallToolRequest, input InputOutput) (result *mcp.CallToolResult, output InputOutput, _ error) {
	return nil, InputOutput{A: input.A, B: input.B, Result: input.A - input.B}, nil

}

func main() {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "SidMCP",
		Version: "0.0.0",
	}, nil)

	// Add Tool
	t := mcp.Tool{
		Name:        "subtract",
		Description: "Subtracts two numbers",
	}
	mcp.AddTool(s, &t, Subtract)

	// Running server over stdin / stdout
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
