package cloud

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	endpoint  string
	accessID  string
	accessKey string
	userID    string

	tokenCachePath string
	http           *http.Client
}

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpireTime   int64  `json:"expire_time"`
	UID          string `json:"uid"`
	ExpiresAt    int64  `json:"expires_at"`
}

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Online   bool   `json:"online"`
}

type Status struct {
	Code  string      `json:"code"`
	Value interface{} `json:"value"`
}

type User struct {
	UID         string `json:"uid"`
	Username    string `json:"username"`
	CountryCode string `json:"country_code"`
	CreateTime  int64  `json:"create_time"`
	UpdateTime  int64  `json:"update_time"`
}

type UserList struct {
	Total    int    `json:"total"`
	List     []User `json:"list"`
	PageNo   int    `json:"page_no"`
	PageSize int    `json:"page_size"`
}

type apiResponse[T any] struct {
	Success bool   `json:"success"`
	Result  T      `json:"result"`
	Msg     string `json:"msg"`
	Code    int    `json:"code"`
}

func New(endpoint, accessID, accessKey, userID string) *Client {
	endpoint = strings.TrimRight(endpoint, "/")
	return &Client{
		endpoint:  endpoint,
		accessID:  accessID,
		accessKey: accessKey,
		userID:    userID,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) SetTokenCachePath(path string) {
	c.tokenCachePath = path
}

func (c *Client) tokenCacheDefaultPath() (string, error) {
	if c.tokenCachePath != "" {
		return c.tokenCachePath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tuya-hub", "token.json"), nil
}

func (c *Client) loadToken() (*Token, error) {
	path, err := c.tokenCacheDefaultPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" || tok.ExpiresAt == 0 {
		return nil, nil
	}
	now := time.Now().Unix()
	if tok.ExpiresAt <= now+30 {
		return nil, nil
	}
	return &tok, nil
}

func (c *Client) saveToken(tok *Token) error {
	path, err := c.tokenCacheDefaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Client) GetToken() (*Token, error) {
	if c.accessID == "" || c.accessKey == "" {
		return nil, errors.New("cloud accessId/accessKey missing")
	}

	if cached, err := c.loadToken(); err == nil && cached != nil {
		return cached, nil
	}

	var result Token
	if err := c.do("GET", "/v1.0/token", url.Values{"grant_type": []string{"1"}}, nil, "", &result); err != nil {
		return nil, err
	}

	result.ExpiresAt = time.Now().Unix() + result.ExpireTime - 60
	if err := c.saveToken(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetDevices() ([]Device, error) {
	tok, err := c.GetToken()
	if err != nil {
		return nil, err
	}

	uid := strings.TrimSpace(c.userID)
	if uid == "" {
		uid = strings.TrimSpace(tok.UID)
	}
	if uid == "" {
		return nil, errors.New("cloud userId missing (set cloud.userId)")
	}

	path := fmt.Sprintf("/v1.0/users/%s/devices", url.PathEscape(uid))
	var result []Device
	if err := c.do("GET", path, nil, nil, tok.AccessToken, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetDeviceStatus(deviceID string) ([]Status, error) {
	tok, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/v1.0/iot-03/devices/%s/status", url.PathEscape(deviceID))
	var result []Status
	if err := c.do("GET", path, nil, nil, tok.AccessToken, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) SendCommands(deviceID string, commands []map[string]any) (map[string]any, error) {
	tok, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"commands": commands}
	path := fmt.Sprintf("/v1.0/iot-03/devices/%s/commands", url.PathEscape(deviceID))
	var result map[string]any
	if err := c.do("POST", path, nil, payload, tok.AccessToken, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetUsers(schema string, pageNo, pageSize int, startTime, endTime int64) (*UserList, error) {
	if strings.TrimSpace(schema) == "" {
		return nil, errors.New("cloud schema missing (set cloud.schema or --schema)")
	}
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if startTime <= 0 || endTime <= 0 {
		endTime = time.Now().Unix()
		startTime = endTime - 30*24*60*60
	}
	tok, err := c.GetToken()
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("page_no", fmt.Sprintf("%d", pageNo))
	query.Set("page_size", fmt.Sprintf("%d", pageSize))
	query.Set("start_time", fmt.Sprintf("%d", startTime))
	query.Set("end_time", fmt.Sprintf("%d", endTime))

	var result UserList
	pathV2 := fmt.Sprintf("/v2.0/apps/%s/users", url.PathEscape(schema))
	if err := c.do("GET", pathV2, query, nil, tok.AccessToken, &result); err == nil {
		return &result, nil
	} else if !strings.Contains(err.Error(), "404") {
		return nil, err
	}
	pathV1 := fmt.Sprintf("/v1.0/apps/%s/users", url.PathEscape(schema))
	if err := c.do("GET", pathV1, query, nil, tok.AccessToken, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) do(method, path string, query url.Values, body any, accessToken string, out any) error {
	reqURL := c.endpoint + path
	if query != nil && len(query) > 0 {
		reqURL = reqURL + "?" + query.Encode()
	}

	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBytes = b
	} else {
		bodyBytes = []byte{}
	}

	contentHash := sha256Hex(bodyBytes)

	pathWithQuery := path
	if query != nil && len(query) > 0 {
		pathWithQuery = path + "?" + query.Encode()
	}

	stringToSign := strings.Join([]string{method, contentHash, "", pathWithQuery}, "\n")

	t := fmt.Sprintf("%d", time.Now().UnixMilli())
	nonce := uuid()
	signStr := c.accessID + t + nonce + stringToSign
	if accessToken != "" {
		signStr = c.accessID + accessToken + t + nonce + stringToSign
	}

	sign := hmacSHA256Upper(signStr, c.accessKey)

	req, err := http.NewRequest(method, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("client_id", c.accessID)
	req.Header.Set("sign", sign)
	req.Header.Set("t", t)
	req.Header.Set("sign_method", "HMAC-SHA256")
	req.Header.Set("nonce", nonce)
	if accessToken != "" {
		req.Header.Set("access_token", accessToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("tuya api error %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	if out == nil {
		return nil
	}

	return decodeResponse(data, out)
}

func decodeResponse(data []byte, out any) error {
	var wrapper struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Msg     string          `json:"msg"`
		Code    int             `json:"code"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	if !wrapper.Success {
		if wrapper.Msg != "" {
			return errors.New(wrapper.Msg)
		}
		return errors.New("tuya api error")
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(wrapper.Result, out)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256Upper(str, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(str))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

func uuid() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
