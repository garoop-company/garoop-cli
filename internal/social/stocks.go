package social

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type StocksClient struct {
	apiKey    string
	apiSecret string
	baseURL   string
	execute   bool
	client    *http.Client
}

type StockOrderResult struct {
	ID       string `json:"id"`
	Symbol   string `json:"symbol"`
	Qty      string `json:"qty"`
	Side     string `json:"side"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	FilledAt string `json:"filled_at"`
}

func NewStocksClient(execute bool) (*StocksClient, error) {
	if !execute {
		return &StocksClient{execute: false}, nil
	}

	key := os.Getenv("ALPACA_API_KEY")
	secret := os.Getenv("ALPACA_API_SECRET")
	if key == "" || secret == "" {
		return nil, fmt.Errorf("株式取引の実行には ALPACA_API_KEY / ALPACA_API_SECRET が必要です")
	}
	baseURL := os.Getenv("ALPACA_BASE_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://paper-api.alpaca.markets"
	}

	return &StocksClient{
		apiKey:    key,
		apiSecret: secret,
		baseURL:   strings.TrimRight(baseURL, "/"),
		execute:   true,
		client:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *StocksClient) PlaceOrder(symbol string, qty int, side, orderType, timeInForce string) (*StockOrderResult, error) {
	if !c.execute {
		return &StockOrderResult{
			ID:     "[dry-run]",
			Symbol: strings.ToUpper(symbol),
			Qty:    fmt.Sprintf("%d", qty),
			Side:   side,
			Type:   orderType,
			Status: "accepted(dry-run)",
		}, nil
	}

	payload := map[string]any{
		"symbol":        strings.ToUpper(symbol),
		"qty":           qty,
		"side":          side,
		"type":          orderType,
		"time_in_force": timeInForce,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v2/orders", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("注文失敗: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out StockOrderResult
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *StocksClient) AccountSummary() (string, error) {
	if !c.execute {
		return "[dry-run] account summary", nil
	}
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v2/account", nil)
	if err != nil {
		return "", err
	}
	c.addHeaders(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("口座取得失敗: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Status      string `json:"status"`
		Cash        string `json:"cash"`
		BuyingPower string `json:"buying_power"`
		Equity      string `json:"equity"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return fmt.Sprintf("status=%s cash=%s buying_power=%s equity=%s", out.Status, out.Cash, out.BuyingPower, out.Equity), nil
}

func (c *StocksClient) Positions() (string, error) {
	if !c.execute {
		return "[dry-run] positions", nil
	}
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v2/positions", nil)
	if err != nil {
		return "", err
	}
	c.addHeaders(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("保有銘柄取得失敗: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

func (c *StocksClient) addHeaders(req *http.Request) {
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)
}
