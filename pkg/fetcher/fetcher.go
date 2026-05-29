package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type ClientConfig struct {
	Timeout time.Duration
}

type Client struct {
	httpClient *http.Client
}

type Page struct {
	URL  string
	Body []byte
}

const defaultTimeout = 90 * time.Second

const browserLikeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

func New(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Fetch(rawURL string) (Page, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, fmt.Errorf("无法访问该网页: %w", err)
	}
	req.Header.Set("User-Agent", browserLikeUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if referer := buildReferer(rawURL); referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("无法访问该网页: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Page{}, fmt.Errorf("无法访问该网页: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Page{}, fmt.Errorf("无法访问该网页: HTTP %s", resp.Status)
	}

	return Page{
		URL:  resp.Request.URL.String(),
		Body: body,
	}, nil
}

func buildReferer(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/"
}
