package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"webtomd/pkg/converter"
	"webtomd/pkg/downloader"
	"webtomd/pkg/fetcher"
	"webtomd/pkg/parser"
	"webtomd/pkg/siteconfig"
)

type Config struct {
	URL            string
	Name           string
	WorkDir        string
	Strict         bool
	SiteConfigPath string
}

func Run(config Config) error {
	urlText := strings.TrimSpace(config.URL)
	if urlText == "" {
		return fmt.Errorf("url is required")
	}
	nameText := strings.TrimSpace(config.Name)
	if err := validateName(nameText); err != nil {
		return err
	}

	workDir := config.WorkDir
	if workDir == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("determine work dir: %w", err)
		}
		workDir = currentDir
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}

	page, err := fetcher.New(fetcher.ClientConfig{}).Fetch(urlText)
	if err != nil {
		return err
	}
	if blocked := detectVerificationPage(page.URL, page.Body); blocked {
		return fmt.Errorf("目标站点触发验证码或环境校验，当前版本不支持自动通过，请在浏览器完成验证后重试")
	}

	parsed, err := parsePage(page.URL, page.Body, config.SiteConfigPath)
	if err != nil {
		return err
	}

	html := parsed.HTML
	if parsed.HasContent && len(parsed.Resources) > 0 {
		resolvedToIndex := make(map[string]int, len(parsed.Resources))
		resources := make([]downloader.Resource, 0, len(parsed.Resources))
		for _, resource := range parsed.Resources {
			if _, exists := resolvedToIndex[resource.ResolvedURL]; exists {
				continue
			}
			resolvedToIndex[resource.ResolvedURL] = len(resources)
			resources = append(resources, downloader.Resource{URL: resource.ResolvedURL})
		}

		assetDir := filepath.Join(workDir, "assets")
		results, err := downloader.New(downloader.Config{
			Concurrency: 5,
			Strict:      config.Strict,
		}).DownloadAll(assetDir, resources)
		if err != nil {
			return err
		}

		replacements := make(map[string]string, len(results)*2)
		for _, resource := range parsed.Resources {
			result := results[resolvedToIndex[resource.ResolvedURL]]
			replacements[resource.OriginalURL] = result.Replacement
			replacements[resource.ResolvedURL] = result.Replacement
		}

		html, err = parser.RewriteResources(html, replacements)
		if err != nil {
			return err
		}
	}

	markdown, err := converter.Convert(converter.Document{
		OriginalURL: page.URL,
		FetchDate:   time.Now(),
		Title:       parsed.Title,
		HTML:        html,
		HasContent:  parsed.HasContent,
	})
	if err != nil {
		return err
	}

	outputPath := filepath.Join(workDir, nameText+".md")
	if err := os.WriteFile(outputPath, []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}

	return nil
}

func parsePage(pageURL string, body []byte, siteConfigPath string) (parser.Result, error) {
	if strings.TrimSpace(siteConfigPath) == "" {
		return parser.Parse(pageURL, body)
	}

	config, err := siteconfig.Load(siteConfigPath)
	if err != nil {
		return parser.Result{}, err
	}

	return parser.ParseWithProfiles(pageURL, body, parser.ProfilesFromConfig(config))
}

func detectVerificationPage(pageURL string, body []byte) bool {
	urlLower := strings.ToLower(pageURL)
	if strings.Contains(urlLower, "wappoc_appmsgcaptcha") {
		return true
	}

	contentLower := strings.ToLower(string(body))
	if strings.Contains(contentLower, "当前环境异常") && strings.Contains(contentLower, "去验证") {
		return true
	}
	if strings.Contains(contentLower, "captcha") && strings.Contains(contentLower, "verify") {
		return true
	}

	return false
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("name must be a file stem")
	}
	if name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("name must not contain path segments")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("name must not contain path separators")
	}
	return nil
}
