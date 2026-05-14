// Package core is a tiny HTTP client for philips-wiz-bulb-core.
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTPClient: &http.Client{Timeout: 20 * time.Second}}
}

type Bulb struct {
	MAC     string `json:"mac"`
	Name    string `json:"name"`
	IP      string `json:"ip"`
	RSSI    *int   `json:"rssi,omitempty"`
	Module  string `json:"module,omitempty"`
	FwVer   string `json:"fw_version,omitempty"`
	State   *bool  `json:"state,omitempty"`
	Dimming *int   `json:"dimming,omitempty"`
	Temp    *int   `json:"temp,omitempty"`
}

type listResp struct {
	Bulbs []Bulb `json:"bulbs"`
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("philips-wiz-bulb-core %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("philips-wiz-bulb-core %s %s: %d: %s", method, path, resp.StatusCode, string(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Client) List(ctx context.Context) ([]Bulb, error) {
	var r listResp
	if err := c.do(ctx, "GET", "/bulbs", nil, &r); err != nil {
		return nil, err
	}
	return r.Bulbs, nil
}

func (c *Client) Get(ctx context.Context, target string) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, "GET", "/bulb/"+target, nil, &out)
}

func (c *Client) Discover(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, "POST", "/discover", map[string]any{"passive": false}, &out)
}

func (c *Client) On(ctx context.Context, target string) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/on", nil, nil)
}
func (c *Client) Off(ctx context.Context, target string) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/off", nil, nil)
}
func (c *Client) Brightness(ctx context.Context, target string, level int) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/brightness", map[string]any{"level": level}, nil)
}
func (c *Client) Temp(ctx context.Context, target string, kelvin int) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/temp", map[string]any{"kelvin": kelvin}, nil)
}
func (c *Client) Color(ctx context.Context, target string, r, g, b int) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/color", map[string]any{"r": r, "g": g, "b": b}, nil)
}
func (c *Client) Scene(ctx context.Context, target, scene string) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/scene", map[string]any{"scene": scene}, nil)
}
func (c *Client) Rename(ctx context.Context, target, newName string) error {
	return c.do(ctx, "POST", "/bulb/"+target+"/name", map[string]any{"name": newName}, nil)
}
