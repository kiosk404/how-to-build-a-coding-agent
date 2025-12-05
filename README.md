# 如何构建一个编程智能代理 (How to Build a Coding Agent)

这是一个完整的 Go 语言教程，受到 [ghuntley/how-to-build-a-coding-agent](https://github.com/ghuntley/how-to-build-a-coding-agent) 项目的启发，教你如何构建一个 Coding Agent，从基础的 Function Call 到基于 MCP (Model Context Protocol) 的编程智能Agent。项目包含从基础对话到高级工具调用的完整示例。

你不需要成为人工智能专家。跟着做，一步步建造！
You don’t need to be an AI expert. Just follow along and build step-by-step!


## 🚀 项目概述

本项目通过一系列渐进式的练习，帮助你掌握构建智能编程代理的核心技术：

- **基础对话** - 与 AI 模型进行简单交互
- **文件操作** - 读取、编辑、搜索文件内容
- **系统工具** - 执行 Bash 命令和文件系统操作
- **MCP 集成** - 使用 Model Context Protocol 扩展 AI 能力
- **多工具协作** - 让 AI 代理能够调用多种外部工具

## 📋 前置要求

### 系统要求
- Go 1.24.4 或更高版本
- Ollama (用于本地 AI 模型运行)

### 安装 Ollama
```bash
# 在 Linux/macOS 上安装
curl -fsSL https://ollama.ai/install.sh | sh

# 在 Windows 上使用 winget
winget install Ollama.Ollama
```

### 下载 AI 模型
```bash
# 下载推荐的模型
ollama pull qwen3:1.7b

# 或者下载其他模型
ollama pull llama3.2:3b
ollama pull deepseek-coder:6.7b
```

## 🛠️ 项目结构
```azure
```


## 🎯 可用练习

### 1. 基础对话 (`chat`)
**学习目标**: 掌握与 AI 模型的基本交互
```bash
go run chat/chat.go --model qwen3:1.7b
```
**示例命令**: 试试和大模型 Say Hi

### 2. 文件读取 (`read`)
**学习目标**: 学习如何读取文件内容
```bash
go run read/read.go --model qwen3:1.7b
```
**示例命令**: "读取一下 read/demo_read.txt 这个文件"

### 3. 文件列表工具 (`list_files`)
**学习目标**: 学习如何列出目录文件
```bash
go run list_files/list_files.go --model qwen3:1.7b
```
**示例命令**: "列出一下当前目录下的所有文件"

### 4. Bash 工具 (`bash_tool`)
**学习目标**: 学习如何执行系统命令
```bash
go run bash_tool/bash_tool.go --model qwen3:1.7b
```
**示例命令**: "执行一下 测试一下网络是否可以连同 www.baidu.com"

### 5. 文件编辑工具 (`edit_tool`)
**学习目标**: 学习如何编辑文件内容
```bash
go run edit_tool/edit_tool.go --model qwen3:1.7b
```
**示例命令**: "编辑一下 read/demo_read.txt 这个文件，把里面的内容替换为 'Hello, World!'"

### 6. MCP 智能代理 (`mcp_agent`)
**学习目标**: 学习使用 MCP 协议构建高级智能代理
```bash
go run mcp_agent/main.go --model qwen3:1.7b
```
**示例命令**: "给我用 Python 在本地写一个冒泡排序"

## 🔧 核心技术

### Model Context Protocol (MCP)
项目使用 MCP 协议让 AI 模型能够调用外部工具：
- **代码搜索工具**: 在代码库中搜索特定内容
- **文件系统工具**: 读写文件和目录操作
- **Web 浏览器工具**: 网页浏览和内容提取

### Ollama 集成
- 支持本地 AI 模型运行
- 流式响应和工具调用
- 多模型兼容

### 工具调用机制
```go
// 工具调用示例
result, err := mcpClient.CallTool(ctx, "read_file", map[string]interface{}{
    "path": "example.txt",
})
```

## 🚀 快速开始

1. **克隆项目**
```bash
git clone https://github.com/kiosk404/how-to-build-a-coding-agent
cd how-to-build-a-coding-agent
```

2. **安装依赖**
```bash
go mod tidy
```

3. **启动主程序**
```bash
go run main.go
```

4. **选择练习**
   程序会自动检测 Ollama 环境并显示所有可用练习。

## 📖 学习路径建议

1. **初学者**: 从 `chat` 和 `read` 开始，了解基础交互
2. **中级用户**: 尝试 `list_files` 和 `bash_tool`，掌握系统操作
3. **高级用户**: 学习 `mcp_agent`，理解 MCP 协议和工具调用

## 🐛 故障排除

### 常见问题

**Q: Ollama 未运行**
```bash
# 启动 Ollama 服务
ollama serve
```

**Q: 没有可用的模型**
```bash
# 下载模型
ollama pull qwen3:1.7b
```

**Q: 模型不支持 Function Call**

一般 qwen 系列的模型都支持 Function Call，但如 gemma3:1b 的模型则不支持。
## 🙏 致谢

- [Ollama](https://ollama.ai) - 本地 AI 模型运行环境
- [Model Context Protocol](https://modelcontextprotocol.io) - 工具调用协议
- [Go](https://golang.org) - 编程语言

---

**开始你的智能代理开发之旅吧！** 🚀
