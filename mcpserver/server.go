package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"box/browser"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var br *browser.Browser

func SetBrowser(b *browser.Browser) {
	br = b
}

type navigateInput struct {
	URL string `json:"url" jsonschema:"URL to navigate to"`
}

type clickInput struct {
	X int `json:"x" jsonschema:"X coordinate"`
	Y int `json:"y" jsonschema:"Y coordinate"`
}

type typeInput struct {
	Text string `json:"text" jsonschema:"Text to type"`
}

type evaluateInput struct {
	Expr string `json:"expr" jsonschema:"JavaScript expression"`
}

type fileReadInput struct {
	Path string `json:"path" jsonschema:"File path to read"`
}

type fileWriteInput struct {
	Path    string `json:"path" jsonschema:"File path to write"`
	Content string `json:"content" jsonschema:"File content"`
}

type terminalExecInput struct {
	Command string `json:"command" jsonschema:"Shell command to execute"`
}

type emptyInput struct{}

func textResult(s string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}, nil, nil
}

func errResult(err error) (*mcp.CallToolResult, any, error) {
	return nil, nil, err
}

func NewMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "drop", Version: "v1.0.0"}, nil)

	mcp.AddTool(s, &mcp.Tool{Name: "browser_navigate", Description: "Navigate browser to URL"}, func(ctx context.Context, req *mcp.CallToolRequest, in navigateInput) (*mcp.CallToolResult, any, error) {
		if br == nil {
			return errResult(fmt.Errorf("browser not started"))
		}
		if err := br.Navigate(in.URL); err != nil {
			return errResult(err)
		}
		return textResult("navigated to " + in.URL)
	})

	mcp.AddTool(s, &mcp.Tool{Name: "browser_click", Description: "Click at coordinates"}, func(ctx context.Context, req *mcp.CallToolRequest, in clickInput) (*mcp.CallToolResult, any, error) {
		if br == nil {
			return errResult(fmt.Errorf("browser not started"))
		}
		if err := br.Click(float64(in.X), float64(in.Y), "left"); err != nil {
			return errResult(err)
		}
		return textResult(fmt.Sprintf("clicked at %d,%d", in.X, in.Y))
	})

	mcp.AddTool(s, &mcp.Tool{Name: "browser_type", Description: "Type text in browser"}, func(ctx context.Context, req *mcp.CallToolRequest, in typeInput) (*mcp.CallToolResult, any, error) {
		if br == nil {
			return errResult(fmt.Errorf("browser not started"))
		}
		if err := br.TypeText(in.Text); err != nil {
			return errResult(err)
		}
		return textResult("typed: " + in.Text)
	})

	mcp.AddTool(s, &mcp.Tool{Name: "browser_screenshot", Description: "Take a screenshot"}, func(ctx context.Context, req *mcp.CallToolRequest, in emptyInput) (*mcp.CallToolResult, any, error) {
		if br == nil {
			return errResult(fmt.Errorf("browser not started"))
		}
		buf, err := br.Screenshot()
		if err != nil {
			return errResult(err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				MIMEType: "image/png",
				Data:     buf,
			}},
		}, nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{Name: "browser_evaluate", Description: "Evaluate JavaScript"}, func(ctx context.Context, req *mcp.CallToolRequest, in evaluateInput) (*mcp.CallToolResult, any, error) {
		if br == nil {
			return errResult(fmt.Errorf("browser not started"))
		}
		res, err := br.Evaluate(in.Expr)
		if err != nil {
			return errResult(err)
		}
		return textResult(res)
	})

	mcp.AddTool(s, &mcp.Tool{Name: "browser_page_info", Description: "Get interactive elements on page"}, func(ctx context.Context, req *mcp.CallToolRequest, in emptyInput) (*mcp.CallToolResult, any, error) {
		if br == nil {
			return errResult(fmt.Errorf("browser not started"))
		}
		info, err := br.GetPageInfo()
		if err != nil {
			return errResult(err)
		}
		return textResult(info)
	})

	mcp.AddTool(s, &mcp.Tool{Name: "file_read", Description: "Read a file"}, func(ctx context.Context, req *mcp.CallToolRequest, in fileReadInput) (*mcp.CallToolResult, any, error) {
		data, err := os.ReadFile(in.Path)
		if err != nil {
			return errResult(err)
		}
		return textResult(string(data))
	})

	mcp.AddTool(s, &mcp.Tool{Name: "file_write", Description: "Write a file"}, func(ctx context.Context, req *mcp.CallToolRequest, in fileWriteInput) (*mcp.CallToolResult, any, error) {
		if err := os.WriteFile(in.Path, []byte(in.Content), 0644); err != nil {
			return errResult(err)
		}
		return textResult("wrote " + in.Path)
	})

	mcp.AddTool(s, &mcp.Tool{Name: "terminal_exec", Description: "Execute a shell command"}, func(ctx context.Context, req *mcp.CallToolRequest, in terminalExecInput) (*mcp.CallToolResult, any, error) {
		cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
		out, err := cmd.CombinedOutput()
		result := string(out)
		if err != nil {
			result += "\nerror: " + err.Error()
		}
		return textResult(result)
	})

	return s
}

func ServeSSE(w http.ResponseWriter, r *http.Request) {
	handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return NewMCPServer()
	}, nil)
	handler.ServeHTTP(w, r)
}

func ServeStreamable() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return NewMCPServer()
	}, nil)
}
