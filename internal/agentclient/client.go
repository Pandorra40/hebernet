package agentclient

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/hebernet/hebernet/internal/protocol"
)

type Client struct {
	SocketPath string
	Timeout    time.Duration
}

func (c *Client) Call(op string, params map[string]any) (map[string]any, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	conn, err := net.DialTimeout("unix", c.SocketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("agent dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	req := protocol.Request{ID: uuid.NewString(), Op: op, Params: params}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}
	var res protocol.Response
	if err := json.NewDecoder(conn).Decode(&res); err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, fmt.Errorf("agent %s: %s", op, res.Error)
	}
	if res.Result == nil {
		res.Result = map[string]any{}
	}
	return res.Result, nil
}
