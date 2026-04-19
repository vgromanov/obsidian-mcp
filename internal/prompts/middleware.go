package prompts

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const methodListPrompts = "prompts/list"
const methodGetPrompt = "prompts/get"

// DynamicVaultMiddleware intercepts prompts/list and prompts/get to serve vault-backed Templater prompts.
func DynamicVaultMiddleware(d Deps) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case methodListPrompts:
				if _, ok := req.(*mcp.ServerRequest[*mcp.ListPromptsParams]); ok {
					return ListFromVault(ctx, d)
				}
			case methodGetPrompt:
				if r, ok := req.(*mcp.ServerRequest[*mcp.GetPromptParams]); ok {
					return GetFromVault(ctx, d, r.Params)
				}
			}
			return next(ctx, method, req)
		}
	}
}
