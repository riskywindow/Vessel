package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/vessel/vessel/internal/paths"
)

// Client connects to the Vessel daemon over the Unix socket.
type Client struct {
	socketPath string
}

// NewClient creates a new daemon client.
func NewClient(socketPath string) *Client {
	if socketPath == "" {
		socketPath = paths.SocketPath
	}
	return &Client{socketPath: socketPath}
}

// Call sends a request to the daemon and returns the response.
func (c *Client) Call(method string, params interface{}, result interface{}) error {
	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(60 * time.Second))

	// Marshal params
	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsJSON = data
	}

	// Send request
	req := Request{
		Method: method,
		Params: paramsJSON,
	}
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	var resp Response
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error
	if resp.Error != nil {
		return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
	}

	// Unmarshal data into result
	if result != nil && resp.Data != nil {
		if err := json.Unmarshal(resp.Data, result); err != nil {
			return fmt.Errorf("failed to unmarshal response data: %w", err)
		}
	}

	return nil
}

// Ping checks if the daemon is responsive.
func (c *Client) Ping() error {
	var result map[string]string
	return c.Call("ping", nil, &result)
}
