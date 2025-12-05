package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// 创建 MCP Server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "filesystem",
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

// ReadFileArgs 定义 read_file 工具的参数
type ReadFileArgs struct {
	Path string `json:"path" mcp:"要读取的文件路径（绝对路径或相对路径）"`
}

// ListDirectoryArgs 定义 list_directory 工具的参数
type ListDirectoryArgs struct {
	Path      string `json:"path" mcp:"要列出内容的目录路径"`
	Recursive bool   `json:"recursive,omitempty" mcp:"是否递归列出子目录内容，默认为 false"`
}

// GetFileInfoArgs 定义 get_file_info 工具的参数
type GetFileInfoArgs struct {
	Path string `json:"path" mcp:"要获取信息的文件或目录路径"`
}

// SearchFilesArgs 定义 search_files 工具的参数
type SearchFilesArgs struct {
	Path    string `json:"path" mcp:"要搜索的目录路径"`
	Pattern string `json:"pattern" mcp:"文件名匹配模式（支持 * 和 ? 通配符）"`
}

// WriteFileArgs 定义 write_file 工具的参数
type WriteFileArgs struct {
	Path    string `json:"path" mcp:"要写入的文件路径（绝对路径或相对路径）"`
	Content string `json:"content" mcp:"要写入的文件内容"`
}

// EditFileArgs 定义 edit_file 工具的参数
type EditFileArgs struct {
	Path    string `json:"path" mcp:"要编辑的文件路径（绝对路径或相对路径）"`
	Content string `json:"content" mcp:"要编辑的文件内容"`
}

// registerTools 注册所有工具
func registerTools(server *mcp.Server) {
	// 1. read_file 工具 - 读取文件内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "read_file",
			Description: "读取指定文件的内容。支持文本文件，返回文件的完整内容。",
		},
		handleReadFile,
	)

	// 2. list_directory 工具 - 列出目录内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_directory",
			Description: "列出指定目录下的所有文件和子目录。",
		},
		handleListDirectory,
	)

	// 3. write_file 工具 - 写入文件内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "write_file",
			Description: "将指定内容写入到文件中。如果文件不存在，会创建新文件；如果文件已存在，会覆盖原有内容。",
		},
		handleWriteFile,
	)

	// 4. edit_file 工具 - 编辑文件内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "edit_file",
			Description: "编辑指定文件的内容。如果文件不存在，会创建新文件；如果文件已存在，会在原有内容基础上进行编辑。",
		},
		handleEditFile,
	)

	// 5. get_file_info 工具 - 获取文件信息
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_file_info",
			Description: "获取文件或目录的详细信息，包括大小、修改时间、权限等。",
		},
		handleGetFileInfo,
	)

	// 6. search_files 工具 - 搜索文件
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_files",
			Description: "在指定目录中搜索匹配模式的文件。",
		},
		handleSearchFiles,
	)
}

// handleReadFile 处理读取文件请求
func handleReadFile(ctx context.Context, req *mcp.CallToolRequest, args ReadFileArgs) (*mcp.CallToolResult, any, error) {
	// 解析路径
	absPath, err := resolvePath(args.Path)
	if err != nil {
		return errorResult(fmt.Sprintf("无法解析路径: %v", err)), nil, nil
	}

	// 检查文件是否存在
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult(fmt.Sprintf("文件不存在: %s", absPath)), nil, nil
		}
		return errorResult(fmt.Sprintf("无法访问文件: %v", err)), nil, nil
	}

	// 检查是否是目录
	if info.IsDir() {
		return errorResult(fmt.Sprintf("%s 是一个目录，不是文件", absPath)), nil, nil
	}

	// 读取文件内容
	content, err := os.ReadFile(absPath)
	if err != nil {
		return errorResult(fmt.Sprintf("读取文件失败: %v", err)), nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(content),
			},
		},
	}, nil, nil
}

// handleListDirectory 处理列出目录请求
func handleListDirectory(ctx context.Context, req *mcp.CallToolRequest, args ListDirectoryArgs) (*mcp.CallToolResult, any, error) {
	// 解析路径
	absPath, err := resolvePath(args.Path)
	if err != nil {
		return errorResult(fmt.Sprintf("无法解析路径: %v", err)), nil, nil
	}

	// 检查目录是否存在
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult(fmt.Sprintf("目录不存在: %s", absPath)), nil, nil
		}
		return errorResult(fmt.Sprintf("无法访问目录: %v", err)), nil, nil
	}

	if !info.IsDir() {
		return errorResult(fmt.Sprintf("%s 不是一个目录", absPath)), nil, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("目录: %s\n\n", absPath))

	if args.Recursive {
		err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的文件
			}
			relPath, _ := filepath.Rel(absPath, path)
			if relPath == "." {
				return nil
			}

			prefix := ""
			if info.IsDir() {
				prefix = "[DIR]  "
			} else {
				prefix = "[FILE] "
			}
			result.WriteString(fmt.Sprintf("%s%s\n", prefix, relPath))
			return nil
		})
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("读取目录失败: %v", err)), nil, nil
		}

		for _, entry := range entries {
			prefix := ""
			if entry.IsDir() {
				prefix = "[DIR]  "
			} else {
				prefix = "[FILE] "
			}
			result.WriteString(fmt.Sprintf("%s%s\n", prefix, entry.Name()))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: result.String(),
			},
		},
	}, nil, nil
}

// handleGetFileInfo 处理获取文件信息请求
func handleGetFileInfo(ctx context.Context, req *mcp.CallToolRequest, args GetFileInfoArgs) (*mcp.CallToolResult, any, error) {
	// 解析路径
	absPath, err := resolvePath(args.Path)
	if err != nil {
		return errorResult(fmt.Sprintf("无法解析路径: %v", err)), nil, nil
	}

	// 获取文件信息
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult(fmt.Sprintf("路径不存在: %s", absPath)), nil, nil
		}
		return errorResult(fmt.Sprintf("无法获取文件信息: %v", err)), nil, nil
	}

	fileType := "文件"
	if info.IsDir() {
		fileType = "目录"
	}

	resultText := fmt.Sprintf(`文件信息:
路径: %s
类型: %s
大小: %d 字节
权限: %s
修改时间: %s
`, absPath, fileType, info.Size(), info.Mode().String(), info.ModTime().Format("2006-01-02 15:04:05"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}, nil, nil
}

// handleSearchFiles 处理搜索文件请求
func handleSearchFiles(ctx context.Context, req *mcp.CallToolRequest, args SearchFilesArgs) (*mcp.CallToolResult, any, error) {
	// 解析路径
	absPath, err := resolvePath(args.Path)
	if err != nil {
		return errorResult(fmt.Sprintf("无法解析路径: %v", err)), nil, nil
	}

	// 检查目录是否存在
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult(fmt.Sprintf("目录不存在: %s", absPath)), nil, nil
		}
		return errorResult(fmt.Sprintf("无法访问目录: %v", err)), nil, nil
	}

	if !info.IsDir() {
		return errorResult(fmt.Sprintf("%s 不是一个目录", absPath)), nil, nil
	}

	var matches []string
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}

		matched, err := filepath.Match(args.Pattern, info.Name())
		if err != nil {
			return nil
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})

	if err != nil {
		return errorResult(fmt.Sprintf("搜索失败: %v", err)), nil, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("搜索结果 (模式: %s):\n\n", args.Pattern))

	if len(matches) == 0 {
		result.WriteString("未找到匹配的文件\n")
	} else {
		for _, match := range matches {
			result.WriteString(match + "\n")
		}
		result.WriteString(fmt.Sprintf("\n共找到 %d 个文件\n", len(matches)))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: result.String(),
			},
		},
	}, nil, nil
}

// handleWriteFile 处理写入文件请求
func handleWriteFile(ctx context.Context, req *mcp.CallToolRequest, args WriteFileArgs) (*mcp.CallToolResult, any, error) {
	// 解析路径
	absPath, err := resolvePath(args.Path)
	if err != nil {
		return errorResult(fmt.Sprintf("无法解析路径: %v", err)), nil, nil
	}

	// 确保目录存在
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errorResult(fmt.Sprintf("无法创建目录: %v", err)), nil, nil
	}

	// 写入文件
	if err := os.WriteFile(absPath, []byte(args.Content), 0644); err != nil {
		return errorResult(fmt.Sprintf("写入文件失败: %v", err)), nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("成功写入文件: %s", absPath),
			},
		},
	}, nil, nil
}

// handleEditFile 处理编辑文件请求
func handleEditFile(ctx context.Context, req *mcp.CallToolRequest, args EditFileArgs) (*mcp.CallToolResult, any, error) {
	// 解析路径
	absPath, err := resolvePath(args.Path)
	if err != nil {
		return errorResult(fmt.Sprintf("无法解析路径: %v", err)), nil, nil
	}
	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return errorResult(fmt.Sprintf("文件不存在: %s", absPath)), nil, nil
	}

	// 编辑文件
	if err := os.WriteFile(absPath, []byte(args.Content), 0644); err != nil {
		return errorResult(fmt.Sprintf("编辑文件失败: %v", err)), nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("成功编辑文件: %s", absPath),
			},
		},
	}, nil, nil
}

// resolvePath 解析路径，支持 ~ 和相对路径
func resolvePath(path string) (string, error) {
	// 处理 ~ 开头的路径
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// errorResult 创建错误结果
func errorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
	}
}
