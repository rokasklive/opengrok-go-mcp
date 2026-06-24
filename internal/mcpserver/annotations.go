// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

// readOnlyToolAnnotations marks tools that only read OpenGrok state (no writes).
var readOnlyToolAnnotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
