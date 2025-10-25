package sifang

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/config"
)

// Client 封装与四方支付平台的 HTTP 通讯
type Client struct {
	baseURL            string
	accessKey          string
	masterKey          string
	defaultMerchantKey string
	merchantKeys       map[int64]string

	httpClient *http.Client
	nowFunc    func() time.Time
}

// Option 自定义客户端行为
type Option func(*Client)

// WithHTTPClient 自定义 HTTP 客户端（测试时使用）
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithNowFunc 自定义时间函数（用于测试）
func WithNowFunc(now func() time.Time) Option {
	return func(c *Client) {
		if now != nil {
			c.nowFunc = now
		}
	}
}

// NewClient 根据配置创建四方支付客户端
func NewClient(cfg config.SifangConfig, opts ...Option) (*Client, error) {
	client := &Client{
		baseURL:            strings.TrimRight(cfg.BaseURL, "/"),
		accessKey:          cfg.AccessKey,
		masterKey:          cfg.MasterKey,
		defaultMerchantKey: cfg.DefaultMerchantKey,
		merchantKeys:       make(map[int64]string, len(cfg.MerchantKeys)),
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		nowFunc: time.Now,
	}

	for id, key := range cfg.MerchantKeys {
		client.merchantKeys[id] = key
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// APIError 表示四方支付业务错误
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("sifang api error: code=%d, message=%s", e.Code, e.Message)
}

// Post 调用指定 action，并将结果解析到 out
// action 例如 "balance"、"orders"
func (c *Client) Post(ctx context.Context, action string, merchantID int64, business map[string]string, out interface{}) error {
	if c.baseURL == "" {
		return fmt.Errorf("sifang baseURL is empty")
	}

	params := make(map[string]string, len(business)+4)
	for k, v := range business {
		params[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	params["merchant_id"] = strconv.FormatInt(merchantID, 10)
	params["timestamp"] = strconv.FormatInt(c.nowFunc().Unix(), 10)

	key, err := c.resolveSigningKey(merchantID)
	if err != nil {
		return err
	}

	if c.shouldUseMasterKey() {
		params["access_key"] = c.accessKey
	}

	sign := computeSign(params, key)
	params["sign"] = sign

	form := url.Values{}
	for k, v := range params {
		if v == "" {
			continue
		}
		form.Set(k, v)
	}

	endpoint := c.buildEndpoint(action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request sifang api failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read sifang response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sifang http error: status=%d, body=%s", resp.StatusCode, truncate(string(body), 256))
	}

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode sifang response failed: %w", err)
	}

	if envelope.Code != 0 {
		return &APIError{Code: envelope.Code, Message: envelope.Message}
	}

	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decode sifang data failed: %w", err)
		}
	}

	return nil
}

func (c *Client) buildEndpoint(action string) string {
	action = strings.Trim(action, "/")
	return fmt.Sprintf("%s/%s", c.baseURL, action)
}

func (c *Client) shouldUseMasterKey() bool {
	return c.accessKey != "" && c.masterKey != ""
}

func (c *Client) resolveSigningKey(merchantID int64) (string, error) {
	if c.shouldUseMasterKey() {
		return c.masterKey, nil
	}

	if key, ok := c.merchantKeys[merchantID]; ok && key != "" {
		return key, nil
	}
	if c.defaultMerchantKey != "" {
		return c.defaultMerchantKey, nil
	}
	return "", fmt.Errorf("sifang merchant key not found for merchant %d", merchantID)
}

func computeSign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "" || strings.TrimSpace(v) == "" {
			continue
		}
		if strings.EqualFold(k, "sign") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for i, key := range keys {
		if i > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(key)
		buf.WriteString("=")
		buf.WriteString(params[key])
	}

	buf.WriteString("&key=")
	buf.WriteString(secret)

	hash := md5.Sum(buf.Bytes())
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
