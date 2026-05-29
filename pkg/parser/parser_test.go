package parser

import (
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseExtractsArticleMedia(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/article.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	result, err := Parse("https://example.com/posts/readable-story", body)
	if err != nil {
		t.Fatalf("parse should not fail: %v", err)
	}

	if !result.HasContent {
		t.Fatal("expected readable article content")
	}
	if !strings.Contains(result.HTML, "The durable web page keeps the essential paragraph") {
		t.Fatalf("expected article body text in cleaned HTML: %q", result.HTML)
	}
	if strings.Contains(result.HTML, "Navigation clutter") {
		t.Fatalf("expected navigation clutter to be removed: %q", result.HTML)
	}
	if strings.Contains(result.HTML, "Footer clutter") {
		t.Fatalf("expected footer clutter to be removed: %q", result.HTML)
	}
	expectedResources := []Resource{
		{
			Type:        ResourceTypeImage,
			OriginalURL: "https://example.com/media/cover.png",
			ResolvedURL: "https://example.com/media/cover.png",
			Attr:        "src",
		},
		{
			Type:        ResourceTypeVideo,
			OriginalURL: "https://cdn.example.com/videos/story.mp4",
			ResolvedURL: "https://cdn.example.com/videos/story.mp4",
			Attr:        "src",
		},
		{
			Type:        ResourceTypeVideo,
			OriginalURL: "https://example.com/media/clip.webm",
			ResolvedURL: "https://example.com/media/clip.webm",
			Attr:        "src",
		},
	}
	if !reflect.DeepEqual(result.Resources, expectedResources) {
		t.Fatalf("unexpected resources:\n got: %#v\nwant: %#v", result.Resources, expectedResources)
	}
}

func TestCollectResourcesIgnoresEmptyAndNonDownloadableSources(t *testing.T) {
	t.Parallel()

	baseURL := mustParseURL(t, "https://example.com/posts/story")
	resources, err := collectResources(baseURL, `
		<article>
			<img src="">
			<img src="   ">
			<img src="data:image/png;base64,abcd">
			<img src="blob:https://example.com/blob-id">
			<img src="javascript:alert(1)">
			<img src="mailto:editor@example.com">
			<img src="#cover">
			<img src="/media/cover.png">
			<video src="/media/movie.mp4"></video>
			<video src="blob:https://example.com/movie.webm"></video>
			<video><source src="clip.webm"></video>
		</article>
	`)
	if err != nil {
		t.Fatalf("collect resources: %v", err)
	}

	expectedResources := []Resource{
		{
			Type:        ResourceTypeImage,
			OriginalURL: "/media/cover.png",
			ResolvedURL: "https://example.com/media/cover.png",
			Attr:        "src",
		},
		{
			Type:        ResourceTypeVideo,
			OriginalURL: "/media/movie.mp4",
			ResolvedURL: "https://example.com/media/movie.mp4",
			Attr:        "src",
		},
		{
			Type:        ResourceTypeVideo,
			OriginalURL: "clip.webm",
			ResolvedURL: "https://example.com/posts/clip.webm",
			Attr:        "src",
		},
	}
	if !reflect.DeepEqual(resources, expectedResources) {
		t.Fatalf("unexpected resources:\n got: %#v\nwant: %#v", resources, expectedResources)
	}
}

func TestParseEmptyReturnsNoContent(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/empty.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	result, err := Parse("https://example.com/app", body)
	if err != nil {
		t.Fatalf("empty shell should not hard fail: %v", err)
	}
	if result.HasContent {
		t.Fatalf("expected no content, got result: %#v", result)
	}
}

func TestParseWeChatUsesJSContentAndDataSrc(t *testing.T) {
	t.Parallel()

	body := []byte(`<!doctype html><html><head><title>fallback title</title></head><body>
		<h1 id="activity-name">微信正文标题</h1>
		<div id="js_content">
			<p>这是微信文章第一段正文，应该被完整提取。</p>
			<p>这是微信文章第二段正文，不应该只剩滑动提示。</p>
			<img src="https://mmbiz.qpic.cn/placeholder/0?wx_fmt=gif" data-src="https://mmbiz.qpic.cn/mmbiz_jpg/abc123/640?wx_fmt=jpeg" />
		</div>
		<div>继续滑动看下一个</div>
	</body></html>`)

	result, err := Parse("https://mp.weixin.qq.com/s/example", body)
	if err != nil {
		t.Fatalf("parse wechat should not fail: %v", err)
	}
	if !result.HasContent {
		t.Fatal("expected wechat content to be extracted")
	}
	if result.Title != "微信正文标题" {
		t.Fatalf("unexpected title: %q", result.Title)
	}
	if !strings.Contains(result.HTML, "这是微信文章第一段正文") {
		t.Fatalf("expected full wechat body text in html, got %q", result.HTML)
	}
	expectedResources := []Resource{
		{
			Type:        ResourceTypeImage,
			OriginalURL: "https://mmbiz.qpic.cn/mmbiz_jpg/abc123/640?wx_fmt=jpeg",
			ResolvedURL: "https://mmbiz.qpic.cn/mmbiz_jpg/abc123/640?wx_fmt=jpeg",
			Attr:        "src",
		},
	}
	if !reflect.DeepEqual(result.Resources, expectedResources) {
		t.Fatalf("unexpected resources:\n got: %#v\nwant: %#v", result.Resources, expectedResources)
	}
}

func TestRewriteResourcesHandlesFragmentInput(t *testing.T) {
	t.Parallel()

	rewritten, err := RewriteResources(
		`<img src="/cover.png" alt="Cover">`,
		map[string]string{"/cover.png": "./assets/cover.png"},
	)
	if err != nil {
		t.Fatalf("rewrite should not fail: %v", err)
	}
	if !strings.Contains(rewritten, `src="./assets/cover.png"`) {
		t.Fatalf("expected rewritten src in output, got %q", rewritten)
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}

	return parsed
}
