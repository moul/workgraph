package main

import "fmt"

// These commands are implemented by the gateway build. Placeholders keep the
// dispatcher complete until the server package lands.

func cmdServe(args []string) error {
	return fmt.Errorf("serve: the gateway is not yet wired in this build")
}

func cmdToken(args []string) error {
	return fmt.Errorf("token: the gateway is not yet wired in this build")
}

func cmdMCP(args []string) error {
	return fmt.Errorf("mcp: the MCP server is not yet wired in this build")
}
