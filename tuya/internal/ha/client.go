package ha

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type State struct {
	EntityID   string         `json:"entity_id"`
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) do(method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ha api error %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (c *Client) States() ([]State, error) {
	data, err := c.do(http.MethodGet, "/api/states", nil)
	if err != nil {
		return nil, err
	}
	var out []State
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) State(entityID string) (*State, error) {
	if strings.TrimSpace(entityID) == "" {
		return nil, errors.New("entity id required")
	}
	data, err := c.do(http.MethodGet, "/api/states/"+entityID, nil)
	if err != nil {
		return nil, err
	}
	var out State
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CallService(domain, service string, payload map[string]any) (map[string]any, error) {
	if domain == "" || service == "" {
		return nil, errors.New("domain and service required")
	}
	data, err := c.do(http.MethodPost, "/api/services/"+domain+"/"+service, payload)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return map[string]any{"result": out}, nil
}

func DomainFromEntity(entityID string) string {
	if strings.Contains(entityID, ".") {
		return strings.SplitN(entityID, ".", 2)[0]
	}
	return ""
}
