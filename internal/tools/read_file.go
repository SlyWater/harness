package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const maxReadBytes = 2 << 20 // 2 MiB

type readFileArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// ReadFile читает текстовый файл и возвращает строки с номерами.
type ReadFile struct{}

func (ReadFile) Name() string { return "read_file" }

func (ReadFile) Description() string {
	return "Read a text file. Returns 1-based line numbers. Use offset and limit to page through large files."
}

func (ReadFile) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path to read"},
			"offset": {"type": "integer", "description": "First line to return, 1-based (default 1)"},
			"limit": {"type": "integer", "description": "Maximum number of lines (default 2000)"}
		},
		"required": ["path"]
	}`)
}

func (ReadFile) Run(ctx context.Context, raw json.RawMessage) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{Output: err.Error(), IsError: true}, nil
	}

	var args readFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{Output: "invalid arguments: " + err.Error(), IsError: true}, nil
	}
	if args.Path == "" {
		return Result{Output: "path is required", IsError: true}, nil
	}

	info, err := os.Stat(args.Path)
	if err != nil {
		return Result{Output: fmt.Sprintf("stat %s: %v", args.Path, err), IsError: true}, nil
	}
	if info.IsDir() {
		return Result{Output: fmt.Sprintf("%s is a directory", args.Path), IsError: true}, nil
	}
	if info.Size() > maxReadBytes {
		return Result{Output: fmt.Sprintf("file %s is too large (%d bytes, limit %d)", args.Path, info.Size(), maxReadBytes), IsError: true}, nil
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return Result{Output: fmt.Sprintf("read %s: %v", args.Path, err), IsError: true}, nil
	}

	text := strings.TrimSuffix(string(data), "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}

	offset := args.Offset
	if offset < 1 {
		offset = 1
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 2000
	}

	start := offset - 1
	if start >= len(lines) {
		return Result{Output: fmt.Sprintf("offset %d is beyond end of file (%d lines)", offset, len(lines))}, nil
	}
	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, "%d: %s\n", i+1, lines[i])
	}
	if end < len(lines) {
		fmt.Fprintf(&b, "... (%d more lines)\n", len(lines)-end)
	}
	return Result{Output: b.String()}, nil
}
