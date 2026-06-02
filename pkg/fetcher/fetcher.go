package fetcher

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type ClientConfig struct {
	Timeout time.Duration
	Cookie  string
}

type Client struct {
	httpClient *http.Client
	cookie     string
}

type Page struct {
	URL  string
	Body []byte
}

const defaultTimeout = 90 * time.Second

const browserLikeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
const mobileBrowserLikeUserAgent = "Mozilla/5.0 (Linux; Android 12; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Mobile Safari/537.36"

func New(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	jar, _ := cookiejar.New(nil)

	return &Client{
		httpClient: &http.Client{Timeout: timeout, Jar: jar},
		cookie:     strings.TrimSpace(config.Cookie),
	}
}

func (c *Client) Fetch(rawURL string) (Page, error) {
	var lastErr error
	for _, candidateURL := range candidateURLsForURL(rawURL) {
		for _, userAgent := range userAgentsForURL(candidateURL) {
			page, err := c.fetchOnce(candidateURL, userAgent)
			if err == nil {
				return page, nil
			}
			lastErr = err
			time.Sleep(300 * time.Millisecond)
		}
	}
	return Page{}, lastErr
}

func (c *Client) fetchOnce(rawURL string, userAgent string) (Page, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, fmt.Errorf("无法访问该网页: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	if referer := buildReferer(rawURL); referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("无法访问该网页: %w", err)
	}
	defer resp.Body.Close()

	reader := io.Reader(resp.Body)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, gzipErr := gzip.NewReader(resp.Body)
		if gzipErr != nil {
			return Page{}, fmt.Errorf("无法访问该网页: %w", gzipErr)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	body, err := io.ReadAll(reader)
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

func userAgentsForURL(rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return []string{browserLikeUserAgent}
	}
	host := parsed.Hostname()
	if host == "weibo.com" || host == "www.weibo.com" || host == "m.weibo.cn" || host == "card.weibo.com" {
		return []string{mobileBrowserLikeUserAgent, browserLikeUserAgent}
	}
	return []string{browserLikeUserAgent}
}

func candidateURLsForURL(rawURL string) []string {
	candidates := []string{rawURL}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return candidates
	}
	host := parsed.Hostname()
	if host != "weibo.com" && host != "www.weibo.com" && host != "card.weibo.com" {
		return candidates
	}
	id := weiboArticleIDFromPath(parsed.Path)
	if id == "" {
		return candidates
	}
	candidates = append(candidates,
		"https://m.weibo.cn/status/"+id,
		"https://card.weibo.com/article/m/show/id/230940"+id,
	)
	return candidates
}

func weiboArticleIDFromPath(path string) string {
	const marker = "/id/"
	index := strings.LastIndex(path, marker)
	if index < 0 {
		return ""
	}
	id := strings.TrimSpace(path[index+len(marker):])
	id = strings.TrimPrefix(id, "230940")
	if id == "" {
		return ""
	}
	return id
}

func buildReferer(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/"
}
