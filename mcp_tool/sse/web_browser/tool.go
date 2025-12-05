package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==================== 参数定义 ====================

// FetchPageArgs 获取网页 HTML 的参数
type FetchPageArgs struct {
	URL     string `json:"url" mcp:"要访问的网页 URL（必填）"`
	Timeout int    `json:"timeout,omitempty" mcp:"超时时间（秒），默认 30 秒"`
}

// GetTextArgs 获取网页文本的参数
type GetTextArgs struct {
	URL      string `json:"url" mcp:"要访问的网页 URL（必填）"`
	Selector string `json:"selector,omitempty" mcp:"CSS 选择器，只获取特定元素的文本（可选）"`
	Timeout  int    `json:"timeout,omitempty" mcp:"超时时间（秒），默认 30 秒"`
}

// GetLinksArgs 获取链接的参数
type GetLinksArgs struct {
	URL     string `json:"url" mcp:"要访问的网页 URL（必填）"`
	Timeout int    `json:"timeout,omitempty" mcp:"超时时间（秒），默认 30 秒"`
}

// ScreenshotArgs 截图的参数
type ScreenshotArgs struct {
	URL      string `json:"url" mcp:"要截图的网页 URL（必填）"`
	FullPage bool   `json:"fullpage,omitempty" mcp:"是否截取完整页面（默认 false，只截取可视区域）"`
	Timeout  int    `json:"timeout,omitempty" mcp:"超时时间（秒），默认 30 秒"`
}

// ==================== 注册工具 ====================

func registerTools(server *mcp.Server) {
	// 1. fetch_page - 获取网页完整 HTML
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "fetch_page",
			Description: "获取网页的完整 HTML 内容。适用于需要分析页面结构的场景。",
		},
		handleFetchPage,
	)

	// 2. get_text - 获取网页纯文本
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_text",
			Description: "获取网页的纯文本内容（去除 HTML 标签）。适用于阅读和理解网页内容。可通过 selector 参数指定只获取特定元素的文本。",
		},
		handleGetText,
	)

	// 3. get_links - 获取页面所有链接
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_links",
			Description: "获取网页中的所有链接。返回链接文本和 URL，方便分析页面导航结构。",
		},
		handleGetLinks,
	)

	// 4. screenshot - 网页截图
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "screenshot",
			Description: "对网页进行截图，返回 base64 编码的 PNG 图片。",
		},
		handleScreenshot,
	)
}

// ==================== 工具处理函数 ====================

// handleFetchPage 获取网页完整 HTML
func handleFetchPage(ctx context.Context, req *mcp.CallToolRequest, args FetchPageArgs) (*mcp.CallToolResult, any, error) {
	if args.URL == "" {
		return errorResult("url 参数不能为空"), nil, nil
	}

	log.Printf("[fetch_page] 开始获取: %s", args.URL)

	timeout := getTimeout(args.Timeout)
	html, err := fetchHTML(args.URL, timeout)
	if err != nil {
		log.Printf("[fetch_page] 失败: %v", err)
		return errorResult("获取网页失败: " + err.Error()), nil, nil
	}

	log.Printf("[fetch_page] 成功，HTML 长度: %d", len(html))
	return textResult(html), nil, nil
}

// handleGetText 获取网页纯文本
func handleGetText(ctx context.Context, req *mcp.CallToolRequest, args GetTextArgs) (*mcp.CallToolResult, any, error) {
	if args.URL == "" {
		return errorResult("url 参数不能为空"), nil, nil
	}

	log.Printf("[get_text] 开始获取: %s, selector: %s", args.URL, args.Selector)

	timeout := getTimeout(args.Timeout)
	text, err := fetchText(args.URL, args.Selector, timeout)
	if err != nil {
		log.Printf("[get_text] 失败: %v", err)
		return errorResult("获取文本失败: " + err.Error()), nil, nil
	}

	log.Printf("[get_text] 成功，文本长度: %d", len(text))
	return textResult(text), nil, nil
}

// handleGetLinks 获取页面所有链接
func handleGetLinks(ctx context.Context, req *mcp.CallToolRequest, args GetLinksArgs) (*mcp.CallToolResult, any, error) {
	if args.URL == "" {
		return errorResult("url 参数不能为空"), nil, nil
	}

	log.Printf("[get_links] 开始获取: %s", args.URL)

	timeout := getTimeout(args.Timeout)
	links, err := fetchLinks(args.URL, timeout)
	if err != nil {
		log.Printf("[get_links] 失败: %v", err)
		return errorResult("获取链接失败: " + err.Error()), nil, nil
	}

	log.Printf("[get_links] 成功，找到 %d 个链接", len(links))

	// 格式化输出
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个链接:\n\n", len(links)))
	for i, link := range links {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, link.Text, link.Href))
	}

	return textResult(sb.String()), nil, nil
}

// handleScreenshot 网页截图
func handleScreenshot(ctx context.Context, req *mcp.CallToolRequest, args ScreenshotArgs) (*mcp.CallToolResult, any, error) {
	if args.URL == "" {
		return errorResult("url 参数不能为空"), nil, nil
	}

	log.Printf("[screenshot] 开始截图: %s, fullpage: %v", args.URL, args.FullPage)

	timeout := getTimeout(args.Timeout)
	imgData, err := takeScreenshot(args.URL, args.FullPage, timeout)
	if err != nil {
		log.Printf("[screenshot] 失败: %v", err)
		return errorResult("截图失败: " + err.Error()), nil, nil
	}

	log.Printf("[screenshot] 成功，图片大小: %d bytes", len(imgData))

	// 返回 base64 编码的图片（作为文本返回，方便 LLM 处理）
	base64Img := base64.StdEncoding.EncodeToString(imgData)
	result := fmt.Sprintf("截图成功！\n\nBase64 编码的 PNG 图片 (data:image/png;base64,...):\n\n%s", base64Img)
	return textResult(result), nil, nil
}

// ==================== 浏览器操作函数 ====================

// Link 表示一个链接
type Link struct {
	Text string `json:"text"`
	Href string `json:"href"`
}

// createBrowserContext 创建浏览器上下文
func createBrowserContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	// 设置 chromedp 选项 - 使用新版 Chrome headless 模式
	// 注意: Chrome 109+ 需要使用 "headless=new" 而不是 "headless"
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,                                // 启用 headless 模式
		chromedp.Flag("headless", "new"),                 // 新版 Chrome headless 模式
		chromedp.Flag("disable-gpu", true),               // 禁用 GPU
		chromedp.Flag("no-sandbox", true),                // 禁用沙箱
		chromedp.Flag("disable-dev-shm-usage", true),     // 禁用 /dev/shm 使用
		chromedp.Flag("disable-web-security", true),      // 禁用 web 安全检查
		chromedp.Flag("ignore-certificate-errors", true), // 忽略证书错误
		chromedp.Flag("disable-extensions", true),        // 禁用扩展
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("mute-audio", true), // 静音
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.WindowSize(1920, 1080), // 设置窗口大小
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	}

	// 检查是否设置了代理
	if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
		log.Printf("[browser] 使用代理: %s", proxy)
		opts = append(opts, chromedp.ProxyServer(proxy))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)

	// 设置超时
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)

	return ctx, func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}
}

// fetchHTML 获取网页 HTML
func fetchHTML(url string, timeout time.Duration) (string, error) {
	ctx, cancel := createBrowserContext(timeout)
	defer cancel()

	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("html", &html),
	)

	return html, err
}

// fetchText 获取网页文本
func fetchText(url, selector string, timeout time.Duration) (string, error) {
	ctx, cancel := createBrowserContext(timeout)
	defer cancel()

	var text string
	var actions []chromedp.Action

	actions = append(actions,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)

	if selector != "" {
		actions = append(actions, chromedp.Text(selector, &text, chromedp.ByQueryAll))
	} else {
		actions = append(actions, chromedp.Text("body", &text))
	}

	err := chromedp.Run(ctx, actions...)
	return text, err
}

// fetchLinks 获取页面链接
func fetchLinks(url string, timeout time.Duration) ([]Link, error) {
	ctx, cancel := createBrowserContext(timeout)
	defer cancel()

	var links []Link

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('a[href]')).map(a => ({
				text: a.innerText.trim().substring(0, 100),
				href: a.href
			})).filter(l => l.text && l.href)
		`, &links),
	)

	return links, err
}

// takeScreenshot 截取网页截图
func takeScreenshot(url string, fullPage bool, timeout time.Duration) ([]byte, error) {
	ctx, cancel := createBrowserContext(timeout)
	defer cancel()

	var imgData []byte
	var actions []chromedp.Action

	actions = append(actions,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Sleep(1*time.Second), // 等待页面渲染
	)

	if fullPage {
		actions = append(actions, chromedp.FullScreenshot(&imgData, 90))
	} else {
		actions = append(actions, chromedp.CaptureScreenshot(&imgData))
	}

	err := chromedp.Run(ctx, actions...)
	return imgData, err
}

// ==================== 辅助函数 ====================

// getTimeout 获取超时时间
func getTimeout(seconds int) time.Duration {
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return DEFAULT_TIMEOUT
}

// textResult 创建文本结果
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}
}

// errorResult 创建错误结果
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: msg,
			},
		},
	}
}
