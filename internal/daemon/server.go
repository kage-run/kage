package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
)

// Server handles connections on the Unix domain socket.
type Server struct {
	listener   net.Listener
	daemon     *Daemon
	logger     *slog.Logger
	socketPath string
}

// NewServer creates a Unix socket server bound to the given path.
func NewServer(socketPath string, d *Daemon, logger *slog.Logger) (*Server, error) {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", socketPath, err)
	}

	// Ensure socket is only accessible by owner
	if err := os.Chmod(socketPath, 0600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("setting socket permissions: %w", err)
	}

	return &Server{
		listener:   listener,
		daemon:     d,
		logger:     logger,
		socketPath: socketPath,
	}, nil
}

// Serve accepts connections until the context is canceled.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.logger.Warn("accept error", "error", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

// Close shuts down the server and removes the socket file.
func (s *Server) Close() error {
	err := s.listener.Close()
	os.Remove(s.socketPath)
	return err
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max message

	if !scanner.Scan() {
		return
	}

	var req Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		s.writeError(conn, "parse_error", fmt.Sprintf("invalid request: %v", err))
		return
	}

	s.logger.Debug("request", "method", req.Method)

	switch req.Method {
	case MethodSandboxStart:
		s.handleSandboxStart(conn, req.Params)
	case MethodSandboxStop:
		s.handleSandboxStop(conn, req.Params)
	case MethodSandboxList:
		s.handleSandboxList(conn)
	case MethodSandboxOutput:
		s.handleSandboxOutput(conn, req.Params)
	case MethodSandboxOutputFollow:
		s.handleSandboxOutputFollow(conn, req.Params)
	case MethodDaemonStop:
		s.handleDaemonStop(conn)
	case MethodDaemonStatus:
		s.handleDaemonStatus(conn)
	default:
		s.writeError(conn, "unknown_method", fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (s *Server) handleSandboxStart(conn net.Conn, params json.RawMessage) {
	var p StartParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.writeError(conn, "invalid_params", err.Error())
		return
	}

	if p.Name == "" {
		s.writeError(conn, "invalid_params", "name is required")
		return
	}
	if len(p.Command) == 0 {
		s.writeError(conn, "invalid_params", "command is required")
		return
	}

	st, err := s.daemon.StartSandbox(p.Name, p.Command, p.Workdir, p.PolicyPath, p.Env)
	if err != nil {
		s.writeError(conn, "start_failed", err.Error())
		return
	}

	s.writeResult(conn, StartResult{
		Name:  p.Name,
		PID:   st.PID,
		State: st.State.String(),
	})
}

func (s *Server) handleSandboxStop(conn net.Conn, params json.RawMessage) {
	var p StopParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.writeError(conn, "invalid_params", err.Error())
		return
	}

	if err := s.daemon.StopSandbox(p.Name); err != nil {
		s.writeError(conn, "stop_failed", err.Error())
		return
	}

	sb, err := s.daemon.GetSandbox(p.Name)
	if err != nil {
		s.writeError(conn, "not_found", err.Error())
		return
	}

	st := sb.Status()
	s.writeResult(conn, StopResult{
		Name:  p.Name,
		State: st.State.String(),
	})
}

func (s *Server) handleSandboxList(conn net.Conn) {
	infos := s.daemon.ListSandboxes()
	s.writeResult(conn, ListResult{Sandboxes: infos})
}

func (s *Server) handleSandboxOutput(conn net.Conn, params json.RawMessage) {
	var p OutputParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.writeError(conn, "invalid_params", err.Error())
		return
	}

	sb, err := s.daemon.GetSandbox(p.Name)
	if err != nil {
		s.writeError(conn, "not_found", err.Error())
		return
	}

	lines := sb.Output(p.Lines)
	s.writeResult(conn, OutputResult{Lines: lines})
}

func (s *Server) handleSandboxOutputFollow(conn net.Conn, params json.RawMessage) {
	var p OutputFollowParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.writeError(conn, "invalid_params", err.Error())
		return
	}

	sb, err := s.daemon.GetSandbox(p.Name)
	if err != nil {
		s.writeError(conn, "not_found", err.Error())
		return
	}

	// Create a context that cancels when the connection closes
	ctx, cancel := context.WithCancel(s.daemon.ctx)
	defer cancel()

	// Detect connection close
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := conn.Read(buf); err != nil {
				cancel()
				return
			}
		}
	}()

	ch := sb.Follow(ctx)
	encoder := json.NewEncoder(conn)
	done := sb.Wait()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			if err := encoder.Encode(line); err != nil {
				return
			}
		case <-done:
			// Process exited — drain remaining buffered lines then close
			for {
				select {
				case line, ok := <-ch:
					if !ok {
						return
					}
					encoder.Encode(line)
				default:
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) handleDaemonStop(conn net.Conn) {
	s.writeResult(conn, map[string]string{"status": "ok"})
	// Trigger shutdown after response is sent
	go s.daemon.cancel()
}

func (s *Server) handleDaemonStatus(conn net.Conn) {
	status := s.daemon.DaemonStatus()
	s.writeResult(conn, status)
}

func (s *Server) writeResult(conn net.Conn, result any) {
	data, _ := json.Marshal(result)
	resp := Response{Result: data}
	line, _ := json.Marshal(resp)
	line = append(line, '\n')
	conn.Write(line)
}

func (s *Server) writeError(conn net.Conn, code, message string) {
	resp := Response{Error: &ErrorResponse{Code: code, Message: message}}
	line, _ := json.Marshal(resp)
	line = append(line, '\n')
	conn.Write(line)
}
