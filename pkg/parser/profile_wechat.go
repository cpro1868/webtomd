package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type weChatProfile struct{}

func (weChatProfile) Match(baseURL *url.URL) bool {
	return strings.EqualFold(baseURL.Host, "mp.weixin.qq.com")
}

func (weChatProfile) Parse(baseURL *url.URL, body []byte) (Result, bool, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return Result{}, false, fmt.Errorf("parse wechat HTML: %w", err)
	}

	content := doc.Find("#js_content").First()
	if content.Length() == 0 {
		return Result{}, false, nil
	}

	content.Find("script, style").Each(func(_ int, selection *goquery.Selection) {
		selection.Remove()
	})
	content.Find("img[data-src]").Each(func(_ int, selection *goquery.Selection) {
		dataSrc, ok := selection.Attr("data-src")
		dataSrc = strings.TrimSpace(dataSrc)
		if ok && dataSrc != "" {
			selection.SetAttr("src", dataSrc)
		}
	})

	html, err := content.Html()
	if err != nil {
		return Result{}, false, fmt.Errorf("render wechat content: %w", err)
	}
	html = strings.TrimSpace(html)
	if html == "" {
		return Result{}, false, nil
	}
	text := strings.TrimSpace(content.Text())
	if text == "" {
		return Result{}, false, nil
	}

	resources, err := collectResources(baseURL, html)
	if err != nil {
		return Result{}, false, err
	}

	title := strings.TrimSpace(doc.Find("#activity-name").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}

	return Result{
		HTML:       html,
		Title:      title,
		Resources:  resources,
		HasContent: true,
	}, true, nil
}
