package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	defaultShellTimeout = 120 * time.Second
	maxShellTimeout     = 10 * time.Minute
	maxOutputBytes      = 32 << 10 // 32 KiB на поток
)

type shellArgs struct {
	Command        string `json:"command"`
	Workdir        string `json:"workdir"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// Shell выполняет команду в системной оболочке.
type Shell struct{}

func (Shell) Name() string { return "shell" }

func (Shell) Description() string {
	return "Run a shell command and return stdout, stderr and exit code. Use for builds, tests and git."
}

func (Shell) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Command to execute"},
			"workdir": {"type": "string", "description": "Working directory (optional)"},
			"timeout_seconds": {"type": "integer", "description": "Timeout in seconds (default 120, max 600)"}
		},
		"required": ["command"]
	}`)
}

func (Shell) Run(ctx context.Context, raw json.RawMessage) (Result, error) {
	var args shellArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{Output: "invalid arguments: " + err.Error(), IsError: true}, nil
	}
	if strings.TrimSpace(args.Command) == "" {
		return Result{Output: "command is required", IsError: true}, nil
	}

	timeout := time.Duration(args.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultShellTimeout
	}
	if timeout > maxShellTimeout {
		timeout = maxShellTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", args.Command)
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", args.Command)
	}
	if args.Workdir != "" {
		cmd.Dir = args.Workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var b strings.Builder
	fmt.Fprintf(&b, "stdout:\n%s\n", truncate(stdout.String(), maxOutputBytes))
	fmt.Fprintf(&b, "stderr:\n%s\n", truncate(stderr.String(), maxOutputBytes))

	isError := false
	if err != nil {
		isError = true
		if runCtx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(&b, "exit: timeout after %s\n", timeout)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(&b, "exit code: %d\n", exitErr.ExitCode())
		} else {
			fmt.Fprintf(&b, "error: %v\n", err)
		}
	} else {
		fmt.Fprintf(&b, "exit code: 0\n")
	}

	return Result{Output: b.String(), IsError: isError}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	half := max / 2
	return s[:half] + "\n...[truncated]...\n" + s[len(s)-half:]
}
