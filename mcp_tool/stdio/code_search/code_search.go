package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	MAX_RESULTS   = 100
	DEFAULT_ROOT  = "."
	MAX_FILE_SIZE = 1024 * 1024
)

var defaultIgnorePatterns = []string{
	".git",
	"node_modules",
	"target",
	"bin",
	"obj",
	"vendor",
	"dist",
	".DS_Store",
}

func main() {
	// 创建 MCP Server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "code_search",
		Version: "1.0.0",
	}, nil)

	// 注册工具
	registerTools(server)

	// 使用 stdio 传输启动服务器
	ctx := context.Background()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

// ==================== 参数定义 ====================

// GrepSearchArgs 正则搜索参数
type GrepSearchArgs struct {
	Pattern    string `json:"pattern" mcp:"搜索模式（正则表达式或普通文本）（必填）"`
	Path       string `json:"path,omitempty" mcp:"搜索的根目录路径（默认为当前目录）"`
	FileType   string `json:"file_type,omitempty" mcp:"限制搜索的文件类型，如 go, py, js（可选）"`
	IgnoreCase bool   `json:"ignore_case,omitempty" mcp:"是否忽略大小写（默认 false）"`
	MaxResults int    `json:"max_results,omitempty" mcp:"最大返回结果数（默认 100）"`
	Context    int    `json:"context,omitempty" mcp:"显示匹配行上下文的行数（默认 0）"`
}

// FindFilesArgs 文件查找参数
type FindFilesArgs struct {
	Pattern    string `json:"pattern" mcp:"文件名匹配模式（支持通配符 * 和 ?）（必填）"`
	Path       string `json:"path,omitempty" mcp:"搜索的根目录路径（默认为当前目录）"`
	MaxResults int    `json:"max_results,omitempty" mcp:"最大返回结果数（默认 100）"`
	Type       string `json:"type,omitempty" mcp:"类型过滤：file 只找文件，dir 只找目录（可选）"`
}

// ReadFileArgs 读取文件参数
type ReadFileArgs struct {
	Path   string `json:"path" mcp:"文件路径（必填）"`
	Offset int    `json:"offset,omitempty" mcp:"起始行号（从 1 开始，默认 1）"`
	Limit  int    `json:"limit,omitempty" mcp:"读取的行数（默认读取全部）"`
}

// ListDirArgs 列出目录参数
type ListDirArgs struct {
	Path      string `json:"path" mcp:"目录路径（必填）"`
	Recursive bool   `json:"recursive,omitempty" mcp:"是否递归列出子目录（默认 false）"`
	MaxDepth  int    `json:"max_depth,omitempty" mcp:"递归时的最大深度（默认 3）"`
}

// SearchSymbolArgs 符号搜索参数
type SearchSymbolArgs struct {
	Symbol   string `json:"symbol" mcp:"要搜索的符号名称（函数名、类名、变量名等）（必填）"`
	Path     string `json:"path,omitempty" mcp:"搜索的根目录路径（默认为当前目录）"`
	FileType string `json:"file_type,omitempty" mcp:"限制搜索的文件类型，如 go, py, js（可选）"`
	Type     string `json:"type,omitempty" mcp:"符号类型：function, class, variable, all（默认 all）"`
}

// ==================== 注册工具 ====================

func registerTools(server *mcp.Server) {
	// 1. grep_search - 正则表达式搜索
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "grep_search",
			Description: "使用正则表达式在代码文件中搜索内容。支持指定文件类型、忽略大小写、显示上下文行。适用于查找特定代码模式、字符串、函数调用等。",
		},
		handleGrepSearch,
	)

	// 2. find_files - 文件名查找
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "find_files",
			Description: "按文件名模式查找文件。支持通配符（* 和 ?）。适用于定位特定文件或某类文件。",
		},
		handleFindFiles,
	)

	// 3. read_file - 读取文件内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "read_file",
			Description: "读取指定文件的内容。支持指定起始行和读取行数。大文件会被截断。",
		},
		handleReadFile,
	)

	// 4. list_dir - 列出目录
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_dir",
			Description: "列出目录中的文件和子目录。支持递归列出和深度控制。返回文件大小和修改时间信息。",
		},
		handleListDir,
	)

	// 5. search_symbol - 符号搜索
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_symbol",
			Description: "搜索代码中的符号定义（函数、类、结构体、接口等）。适用于快速定位代码定义。",
		},
		handleSearchSymbol,
	)
}

// ==================== 工具处理函数 ====================

// handleGrepSearch 处理正则搜索
func handleGrepSearch(ctx context.Context, req *mcp.CallToolRequest, args GrepSearchArgs) (*mcp.CallToolResult, any, error) {
	if args.Pattern == "" {
		return errorResult("pattern 参数不能为空"), nil, nil
	}

	// grep_search: 搜索模式, 路径, 文件类型

	rootPath := args.Path
	if rootPath == "" {
		rootPath = DEFAULT_ROOT
	}

	// 尝试使用系统 ripgrep (rg) 命令，如果不存在则使用内置实现
	results, err := grepWithRipgrep(args, rootPath)
	if err != nil {
		// ripgrep 不可用，使用内置搜索
		results, err = grepBuiltin(args, rootPath)
		if err != nil {
			// 搜索失败
			return errorResult("搜索失败: " + err.Error()), nil, nil
		}
	}

	// 找到匹配结果

	if len(results) == 0 {
		return textResult("未找到匹配的结果"), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个匹配:\n\n", len(results)))
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("📄 %s:%d\n", r.File, r.Line))
		sb.WriteString(fmt.Sprintf("   %s\n\n", strings.TrimSpace(r.Content)))
	}

	return textResult(sb.String()), nil, nil
}

// handleFindFiles 处理文件查找
func handleFindFiles(ctx context.Context, req *mcp.CallToolRequest, args FindFilesArgs) (*mcp.CallToolResult, any, error) {
	if args.Pattern == "" {
		return errorResult("pattern 参数不能为空"), nil, nil
	}

	// find_files: 查找文件

	rootPath := args.Path
	if rootPath == "" {
		rootPath = DEFAULT_ROOT
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = MAX_RESULTS
	}

	// 将通配符模式转换为正则表达式
	regexPattern := wildcardToRegex(args.Pattern)
	re, err := regexp.Compile("(?i)" + regexPattern) // 忽略大小写
	if err != nil {
		return errorResult("无效的文件名模式: " + err.Error()), nil, nil
	}

	var files []FileInfo
	count := 0

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		// 检查是否应该忽略
		if shouldIgnore(path, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 类型过滤
		if args.Type == "file" && d.IsDir() {
			return nil
		}
		if args.Type == "dir" && !d.IsDir() {
			return nil
		}

		// 匹配文件名
		if re.MatchString(d.Name()) {
			info, _ := d.Info()
			var size int64
			var modTime time.Time
			if info != nil {
				size = info.Size()
				modTime = info.ModTime()
			}

			files = append(files, FileInfo{
				Path:    path,
				Name:    d.Name(),
				IsDir:   d.IsDir(),
				Size:    size,
				ModTime: modTime,
			})

			count++
			if count >= maxResults {
				return filepath.SkipAll
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return errorResult("查找文件失败: " + err.Error()), nil, nil
	}

	// 找到文件

	if len(files) == 0 {
		return textResult("未找到匹配的文件"), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个匹配:\n\n", len(files)))
	for _, f := range files {
		icon := "📄"
		if f.IsDir {
			icon = "📁"
		}
		sb.WriteString(fmt.Sprintf("%s %s", icon, f.Path))
		if !f.IsDir && f.Size > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", formatSize(f.Size)))
		}
		sb.WriteString("\n")
	}

	return textResult(sb.String()), nil, nil
}

// handleReadFile 处理文件读取
func handleReadFile(ctx context.Context, req *mcp.CallToolRequest, args ReadFileArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return errorResult("path 参数不能为空"), nil, nil
	}

	// read_file: 读取文件

	// 检查文件是否存在
	info, err := os.Stat(args.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult("文件不存在: " + args.Path), nil, nil
		}
		return errorResult("无法访问文件: " + err.Error()), nil, nil
	}

	if info.IsDir() {
		return errorResult("指定的路径是目录，不是文件"), nil, nil
	}

	// 检查文件大小
	if info.Size() > MAX_FILE_SIZE {
		return errorResult(fmt.Sprintf("文件太大 (%s)，超过限制 (%s)。请使用 offset 和 limit 参数分段读取。",
			formatSize(info.Size()), formatSize(MAX_FILE_SIZE))), nil, nil
	}

	// 读取文件
	file, err := os.Open(args.Path)
	if err != nil {
		return errorResult("打开文件失败: " + err.Error()), nil, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0
	offset := args.Offset
	if offset <= 0 {
		offset = 1
	}

	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if args.Limit > 0 && len(lines) >= args.Limit {
			break
		}
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return errorResult("读取文件失败: " + err.Error()), nil, nil
	}

	// 成功读取文件

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📄 %s (第 %d-%d 行，共 %d 行)\n\n", args.Path, offset, offset+len(lines)-1, lineNum))
	for i, line := range lines {
		sb.WriteString(fmt.Sprintf("%4d | %s\n", offset+i, line))
	}

	return textResult(sb.String()), nil, nil
}

// handleListDir 处理目录列出
func handleListDir(ctx context.Context, req *mcp.CallToolRequest, args ListDirArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return errorResult("path 参数不能为空"), nil, nil
	}

	// list_dir: 列出目录

	// 检查目录是否存在
	info, err := os.Stat(args.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult("目录不存在: " + args.Path), nil, nil
		}
		return errorResult("无法访问目录: " + err.Error()), nil, nil
	}

	if !info.IsDir() {
		return errorResult("指定的路径不是目录"), nil, nil
	}

	maxDepth := args.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	var items []FileInfo
	basePath := filepath.Clean(args.Path)
	baseDepth := strings.Count(basePath, string(filepath.Separator))

	err = filepath.WalkDir(args.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// 跳过根目录本身
		if path == args.Path {
			return nil
		}

		// 检查是否应该忽略
		if shouldIgnore(path, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 计算深度
		currentDepth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - baseDepth

		// 非递归模式只列出第一层
		if !args.Recursive && currentDepth > 1 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 递归模式检查深度限制
		if args.Recursive && currentDepth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, _ := d.Info()
		var size int64
		var modTime time.Time
		if info != nil {
			size = info.Size()
			modTime = info.ModTime()
		}

		// 计算相对路径
		relPath, _ := filepath.Rel(args.Path, path)

		items = append(items, FileInfo{
			Path:    relPath,
			Name:    d.Name(),
			IsDir:   d.IsDir(),
			Size:    size,
			ModTime: modTime,
		})

		return nil
	})

	if err != nil {
		return errorResult("列出目录失败: " + err.Error()), nil, nil
	}

	// 排序：目录在前，然后按名称排序
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	// 找到项目

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 %s (%d 项)\n\n", args.Path, len(items)))

	for _, item := range items {
		icon := "📄"
		if item.IsDir {
			icon = "📁"
		}
		sb.WriteString(fmt.Sprintf("%s %s", icon, item.Path))
		if !item.IsDir && item.Size > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", formatSize(item.Size)))
		}
		sb.WriteString("\n")
	}

	return textResult(sb.String()), nil, nil
}

// handleSearchSymbol 处理符号搜索
func handleSearchSymbol(ctx context.Context, req *mcp.CallToolRequest, args SearchSymbolArgs) (*mcp.CallToolResult, any, error) {
	if args.Symbol == "" {
		return errorResult("symbol 参数不能为空"), nil, nil
	}

	// search_symbol: 搜索符号

	rootPath := args.Path
	if rootPath == "" {
		rootPath = DEFAULT_ROOT
	}

	// 根据文件类型构建符号定义的正则表达式
	patterns := buildSymbolPatterns(args.Symbol, args.FileType, args.Type)

	var results []SearchResult

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if shouldIgnore(path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查文件类型
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if args.FileType != "" && ext != args.FileType {
			return nil
		}

		// 只搜索代码文件
		if !isCodeFile(path) {
			return nil
		}

		// 在文件中搜索符号
		fileResults, err := searchSymbolInFile(path, patterns)
		if err != nil {
			return nil
		}

		results = append(results, fileResults...)

		if len(results) >= MAX_RESULTS {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return errorResult("搜索符号失败: " + err.Error()), nil, nil
	}

	// 找到符号定义

	if len(results) == 0 {
		return textResult("未找到符号定义: " + args.Symbol), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个符号定义:\n\n", len(results)))
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("📍 %s:%d [%s]\n", r.File, r.Line, r.Type))
		sb.WriteString(fmt.Sprintf("   %s\n\n", strings.TrimSpace(r.Content)))
	}

	return textResult(sb.String()), nil, nil
}

// ==================== 辅助类型和函数 ====================

// SearchResult 搜索结果
type SearchResult struct {
	File    string
	Line    int
	Content string
	Type    string // 用于符号搜索时标识类型
}

// FileInfo 文件信息
type FileInfo struct {
	Path    string
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// grepWithRipgrep 使用 ripgrep 进行搜索
func grepWithRipgrep(args GrepSearchArgs, rootPath string) ([]SearchResult, error) {
	// 检查 rg 是否可用
	_, err := exec.LookPath("rg")
	if err != nil {
		return nil, err
	}

	cmdArgs := []string{
		"--line-number",
		"--no-heading",
		"--color=never",
	}

	if args.IgnoreCase {
		cmdArgs = append(cmdArgs, "--ignore-case")
	}

	if args.FileType != "" {
		cmdArgs = append(cmdArgs, "--type", args.FileType)
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = MAX_RESULTS
	}
	cmdArgs = append(cmdArgs, "--max-count", fmt.Sprintf("%d", maxResults))

	if args.Context > 0 {
		cmdArgs = append(cmdArgs, "--context", fmt.Sprintf("%d", args.Context))
	}

	cmdArgs = append(cmdArgs, args.Pattern, rootPath)

	cmd := exec.Command("rg", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		// rg 返回非零退出码可能只是没找到结果
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []SearchResult{}, nil
		}
		return nil, err
	}

	return parseRipgrepOutput(string(output))
}

// parseRipgrepOutput 解析 ripgrep 输出
func parseRipgrepOutput(output string) ([]SearchResult, error) {
	var results []SearchResult
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// 格式: file:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		lineNum := 0
		fmt.Sscanf(parts[1], "%d", &lineNum)

		results = append(results, SearchResult{
			File:    parts[0],
			Line:    lineNum,
			Content: parts[2],
		})
	}

	return results, nil
}

// grepBuiltin 内置搜索实现
func grepBuiltin(args GrepSearchArgs, rootPath string) ([]SearchResult, error) {
	pattern := args.Pattern
	if args.IgnoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("无效的正则表达式: %v", err)
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = MAX_RESULTS
	}

	var results []SearchResult

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if shouldIgnore(path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查文件类型
		if args.FileType != "" {
			ext := strings.TrimPrefix(filepath.Ext(path), ".")
			if ext != args.FileType {
				return nil
			}
		}

		// 只搜索文本文件
		if !isTextFile(path) {
			return nil
		}

		// 在文件中搜索
		fileResults, err := searchInFile(path, re, maxResults-len(results))
		if err != nil {
			return nil
		}

		results = append(results, fileResults...)

		if len(results) >= maxResults {
			return filepath.SkipAll
		}

		return nil
	})

	return results, err
}

// searchInFile 在文件中搜索
func searchInFile(path string, re *regexp.Regexp, maxResults int) ([]SearchResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []SearchResult
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			results = append(results, SearchResult{
				File:    path,
				Line:    lineNum,
				Content: line,
			})

			if len(results) >= maxResults {
				break
			}
		}
	}

	return results, scanner.Err()
}

// searchSymbolInFile 在文件中搜索符号
func searchSymbolInFile(path string, patterns []*regexp.Regexp) ([]SearchResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []SearchResult
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, re := range patterns {
			if re.MatchString(line) {
				symbolType := detectSymbolType(line, filepath.Ext(path))
				results = append(results, SearchResult{
					File:    path,
					Line:    lineNum,
					Content: line,
					Type:    symbolType,
				})
				break
			}
		}
	}

	return results, scanner.Err()
}

// buildSymbolPatterns 构建符号搜索的正则表达式
func buildSymbolPatterns(symbol, fileType, symbolType string) []*regexp.Regexp {
	var patterns []string
	escapedSymbol := regexp.QuoteMeta(symbol)

	// Go 语言模式
	if fileType == "" || fileType == "go" {
		patterns = append(patterns,
			fmt.Sprintf(`func\s+(\([^)]+\)\s+)?%s\s*\(`, escapedSymbol),  // 函数/方法定义
			fmt.Sprintf(`type\s+%s\s+(struct|interface)`, escapedSymbol), // 结构体/接口定义
			fmt.Sprintf(`var\s+%s\s+`, escapedSymbol),                    // 变量定义
			fmt.Sprintf(`const\s+%s\s+`, escapedSymbol),                  // 常量定义
		)
	}

	// Python 语言模式
	if fileType == "" || fileType == "py" {
		patterns = append(patterns,
			fmt.Sprintf(`def\s+%s\s*\(`, escapedSymbol),      // 函数定义
			fmt.Sprintf(`class\s+%s\s*[:\(]`, escapedSymbol), // 类定义
			fmt.Sprintf(`%s\s*=`, escapedSymbol),             // 变量赋值
		)
	}

	// JavaScript/TypeScript 语言模式
	if fileType == "" || fileType == "js" || fileType == "ts" || fileType == "tsx" || fileType == "jsx" {
		patterns = append(patterns,
			fmt.Sprintf(`function\s+%s\s*\(`, escapedSymbol),       // 函数定义
			fmt.Sprintf(`(const|let|var)\s+%s\s*=`, escapedSymbol), // 变量定义
			fmt.Sprintf(`class\s+%s\s*`, escapedSymbol),            // 类定义
			fmt.Sprintf(`%s\s*:\s*function`, escapedSymbol),        // 对象方法
			fmt.Sprintf(`%s\s*=\s*\(.*\)\s*=>`, escapedSymbol),     // 箭头函数
		)
	}

	// Java 语言模式
	if fileType == "" || fileType == "java" {
		patterns = append(patterns,
			fmt.Sprintf(`(public|private|protected)?\s*(static)?\s*\w+\s+%s\s*\(`, escapedSymbol), // 方法定义
			fmt.Sprintf(`class\s+%s\s*`, escapedSymbol),                                           // 类定义
			fmt.Sprintf(`interface\s+%s\s*`, escapedSymbol),                                       // 接口定义
		)
	}

	// Rust 语言模式
	if fileType == "" || fileType == "rs" {
		patterns = append(patterns,
			fmt.Sprintf(`fn\s+%s\s*[<\(]`, escapedSymbol), // 函数定义
			fmt.Sprintf(`struct\s+%s\s*`, escapedSymbol),  // 结构体定义
			fmt.Sprintf(`trait\s+%s\s*`, escapedSymbol),   // trait 定义
			fmt.Sprintf(`impl\s+%s\s*`, escapedSymbol),    // impl 定义
		)
	}

	var compiled []*regexp.Regexp
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return compiled
}

// detectSymbolType 检测符号类型
func detectSymbolType(line, ext string) string {
	line = strings.TrimSpace(line)

	switch ext {
	case ".go":
		if strings.HasPrefix(line, "func") {
			return "function"
		}
		if strings.Contains(line, "struct") {
			return "struct"
		}
		if strings.Contains(line, "interface") {
			return "interface"
		}
		if strings.HasPrefix(line, "var") {
			return "variable"
		}
		if strings.HasPrefix(line, "const") {
			return "constant"
		}
	case ".py":
		if strings.HasPrefix(line, "def") {
			return "function"
		}
		if strings.HasPrefix(line, "class") {
			return "class"
		}
	case ".js", ".ts", ".jsx", ".tsx":
		if strings.Contains(line, "function") || strings.Contains(line, "=>") {
			return "function"
		}
		if strings.Contains(line, "class") {
			return "class"
		}
	case ".java":
		if strings.Contains(line, "class") {
			return "class"
		}
		if strings.Contains(line, "interface") {
			return "interface"
		}
		return "method"
	case ".rs":
		if strings.HasPrefix(line, "fn") {
			return "function"
		}
		if strings.HasPrefix(line, "struct") {
			return "struct"
		}
		if strings.HasPrefix(line, "trait") {
			return "trait"
		}
	}

	return "symbol"
}

// shouldIgnore 检查是否应该忽略
func shouldIgnore(path, name string) bool {
	for _, pattern := range defaultIgnorePatterns {
		if strings.HasPrefix(pattern, "*.") {
			// 扩展名匹配
			if strings.HasSuffix(name, pattern[1:]) {
				return true
			}
		} else if name == pattern {
			return true
		}
	}
	return false
}

// isTextFile 检查是否是文本文件
func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rs": true, ".rb": true, ".php": true, ".swift": true, ".kt": true,
		".scala": true, ".cs": true, ".vb": true, ".lua": true, ".pl": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".html": true, ".css": true, ".scss": true, ".less": true,
		".xml": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
		".md": true, ".txt": true, ".log": true, ".conf": true, ".cfg": true,
		".ini": true, ".env": true, ".sql": true, ".graphql": true,
		".proto": true, ".thrift": true, ".vue": true, ".svelte": true,
		".makefile": true, ".dockerfile": true, ".gitignore": true,
	}
	return textExts[ext]
}

// isCodeFile 检查是否是代码文件
func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rs": true, ".rb": true, ".php": true, ".swift": true, ".kt": true,
		".scala": true, ".cs": true, ".vb": true, ".lua": true, ".pl": true,
	}
	return codeExts[ext]
}

// wildcardToRegex 将通配符模式转换为正则表达式
func wildcardToRegex(pattern string) string {
	// 转义特殊字符
	pattern = regexp.QuoteMeta(pattern)
	// 将 \* 替换为 .*
	pattern = strings.ReplaceAll(pattern, `\*`, `.*`)
	// 将 \? 替换为 .
	pattern = strings.ReplaceAll(pattern, `\?`, `.`)
	return "^" + pattern + "$"
}

// formatSize 格式化文件大小
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
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
