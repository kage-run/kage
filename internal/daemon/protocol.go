package daemon

import "encoding/json"

// Method constants for the daemon socket API.
const (
	MethodSandboxStart        = "sandbox.start"
	MethodSandboxStop         = "sandbox.stop"
	MethodSandboxList         = "sandbox.list"
	MethodSandboxOutput       = "sandbox.output"
	MethodSandboxOutputFollow = "sandbox.output_follow"
	MethodDaemonStop          = "daemon.stop"
	MethodDaemonStatus        = "daemon.status"
)

// Request is the JSON message sent from CLI to daemon over the Unix socket.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is the JSON message sent from daemon back to CLI.
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorResponse  `json:"error,omitempty"`
}

// ErrorResponse carries error details in a Response.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Param and result types for each method ---

type StartParams struct {
	Name       string   `json:"name"`
	Command    []string `json:"command"`
	Workdir    string   `json:"workdir"`
	PolicyPath string   `json:"policy_path"`
	Env        []string `json:"env"`
}

type StartResult struct {
	Name  string `json:"name"`
	PID   int    `json:"pid"`
	State string `json:"state"`
}

type StopParams struct {
	Name string `json:"name"`
}

type StopResult struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type StatusParams struct {
	Name string `json:"name"`
}

type SandboxInfo struct {
	Name      string `json:"name"`
	PID       int    `json:"pid"`
	State     string `json:"state"`
	ExitCode  int    `json:"exit_code"`
	StartedAt string `json:"started_at,omitempty"`
	Uptime    string `json:"uptime,omitempty"`
}

type ListResult struct {
	Sandboxes []SandboxInfo `json:"sandboxes"`
}

type OutputParams struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
}

type OutputResult struct {
	Lines []string `json:"lines"`
}

type OutputFollowParams struct {
	Name string `json:"name"`
}

type DaemonStatusResult struct {
	Version      string `json:"version"`
	Uptime       string `json:"uptime"`
	SandboxCount int    `json:"sandbox_count"`
}
