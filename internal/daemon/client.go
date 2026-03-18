package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// ErrDaemonNotRunning indicates the daemon socket doesn't exist or can't be reached.
var ErrDaemonNotRunning = errors.New("daemon is not running")

// Client is a thin client that communicates with the daemon over a Unix socket.
type Client struct {
	SocketPath string
}

// NewClient creates a client that connects to the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{SocketPath: socketPath}
}

// Call sends a request to the daemon and decodes the response into result.
func (c *Client) Call(method string, params any, result any) error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Set a reasonable deadline for non-streaming calls
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	if err := c.sendRequest(conn, method, params); err != nil {
		return err
	}

	return c.readResponse(conn, result)
}

// Stream sends a request and returns the raw connection for streaming reads.
// The caller is responsible for closing the returned ReadCloser.
func (c *Client) Stream(method string, params any) (io.ReadCloser, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}

	if err := c.sendRequest(conn, method, params); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (c *Client) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", c.SocketPath, 3*time.Second)
	if err != nil {
		return nil, ErrDaemonNotRunning
	}
	return conn, nil
}

func (c *Client) sendRequest(conn net.Conn, method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("encoding params: %w", err)
		}
	}

	req := Request{
		Method: method,
		Params: rawParams,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	return nil
}

func (c *Client) readResponse(conn net.Conn, result any) error {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
		return fmt.Errorf("empty response from daemon")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
	}

	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("decoding result: %w", err)
		}
	}

	return nil
}
