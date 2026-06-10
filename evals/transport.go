// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectStdio(ctx context.Context, bin string, args, env []string) (*mcp.ClientSession, error) {
	c := exec.CommandContext(ctx, bin, args...)
	c.Env = append(os.Environ(), env...)
	c.Stderr = os.Stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "eval-harness", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: c}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect stdio: %w", err)
	}
	return session, nil
}
