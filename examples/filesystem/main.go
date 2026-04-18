// Package main implements a real-world MCP server for filesystem operations.
// This serves as both an integration test and a practical example.
//
// Usage with Claude Desktop:
//
//	{
//	  "mcpServers": {
//	    "filesystem": {
//	      "command": "go",
//	      "args": ["run", "./examples/filesystem"],
//	      "env": { "FS_ROOT": "/tmp/mcp-test" }
//	    }
//	  }
//	}
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhangpanda/gomcp"
)

type ReadFileInput struct {
	Path string `json:"path" mcp:"required,desc=File path relative to root"`
}

type WriteFileInput struct {
	Path    string `json:"path" mcp:"required,desc=File path relative to root"`
	Content string `json:"content" mcp:"required,desc=File content to write"`
}

type ListDirInput struct {
	Path string `json:"path" mcp:"desc=Directory path relative to root (empty for root)"`
}

type SearchInput struct {
	Pattern string `json:"pattern" mcp:"required,desc=Glob pattern to search for"`
}

type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"isDir"`
	ModTime string `json:"modTime"`
}

var root string

func init() {
	root = os.Getenv("FS_ROOT")
	if root == "" {
		root = "/tmp/mcp-test"
	}
	os.MkdirAll(root, 0o755)
}

func safePath(rel string) (string, error) {
	abs := filepath.Join(root, filepath.Clean(rel))
	if !strings.HasPrefix(abs, root) {
		return "", fmt.Errorf("path traversal not allowed")
	}
	return abs, nil
}

func main() {
	s := gomcp.New("filesystem", "1.0.0")

	s.Use(gomcp.Recovery())
	s.Use(gomcp.Logger())
	s.Use(gomcp.Timeout(10 * time.Second))

	// --- Tools ---

	s.ToolFunc("read_file", "Read a file's content", func(ctx *gomcp.Context, in ReadFileInput) (string, error) {
		path, err := safePath(in.Path)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", in.Path, err)
		}
		return string(data), nil
	})

	s.ToolFunc("write_file", "Write content to a file", func(ctx *gomcp.Context, in WriteFileInput) (string, error) {
		path, err := safePath(in.Path)
		if err != nil {
			return "", err
		}
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", in.Path, err)
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.Path), nil
	})

	s.ToolFunc("list_dir", "List directory contents", func(ctx *gomcp.Context, in ListDirInput) ([]FileInfo, error) {
		path, err := safePath(in.Path)
		if err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", in.Path, err)
		}
		var files []FileInfo
		for _, e := range entries {
			info, _ := e.Info()
			if info == nil {
				continue
			}
			files = append(files, FileInfo{
				Name:    e.Name(),
				Size:    info.Size(),
				IsDir:   e.IsDir(),
				ModTime: info.ModTime().Format(time.RFC3339),
			})
		}
		return files, nil
	})

	s.ToolFunc("search_files", "Search for files matching a glob pattern", func(ctx *gomcp.Context, in SearchInput) ([]string, error) {
		matches, err := filepath.Glob(filepath.Join(root, in.Pattern))
		if err != nil {
			return nil, err
		}
		var rel []string
		for _, m := range matches {
			r, _ := filepath.Rel(root, m)
			rel = append(rel, r)
		}
		return rel, nil
	})

	s.Tool("get_info", "Get filesystem root info", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.JSON(map[string]string{"root": root}), nil
	})

	// --- Resource ---
	s.Resource("fs://root", "Filesystem root path", func(ctx *gomcp.Context) (any, error) {
		return map[string]string{"root": root}, nil
	})

	// --- Prompt ---
	s.Prompt("file_review", "Review a file",
		[]gomcp.PromptArgument{gomcp.PromptArg("path", "File path", true)},
		func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
			return []gomcp.PromptMessage{
				gomcp.UserMsg(fmt.Sprintf("Please review the file at %s for issues.", ctx.String("path"))),
			}, nil
		},
	)

	log.Println("Starting filesystem MCP server, root:", root)
	if err := s.Stdio(); err != nil {
		log.Fatal(err)
	}
}
