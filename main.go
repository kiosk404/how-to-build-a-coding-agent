package main

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	Bold        = "\033[1m"
)

func main() {
	models := checkOllamaEnvironment()
	if models == nil {
		return
	}

	fmt.Printf("\n%s%s═══════════════════════════════════%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("%s%s       5. Build a Coding Agent%s\n", Bold, ColorCyan, ColorReset)
	fmt.Printf("%s%s═══════════════════════════════════%s\n\n", Bold, ColorBlue, ColorReset)

	exercises := []struct {
		Name        string
		Description string
		Path        string
	}{
		{"chat", "基础对话 - 学习如何和AI进行简单对话， 试试和大模型Say Hi", "chat/chat.go"},
		{"read", "文件读取 - 学习如何读取文件内容，试试和大模型Say '读取一下 read/demo_read.txt 这个文件'", "read/read.go"},
		{"list_files", "文件列表工具 - 学习如何列出当前目录下的所有文件， 试试和大模型Say '列出一下当前目录下的所有文件'", "list_files/list_files.go"},
		{"bash_tool", "Bash工具 - 学习如何使用Bash工具， 试试和大模型Say '执行一下 测试一下网络是否可以连同 www.baidu.com'", "bash_tool/bash_tool.go"},
		{"edit_tool", "文件编辑工具 - 学习如何使用文件编辑工具， 试试和大模型Say '编辑一下 read/demo_read.txt 这个文件， 把里面的内容替换为 'Hello, World!''", "edit_tool/edit_tool.go"},
		{"code_search_tool", "代码搜索工具 - 学习如何使用代码搜索工具， 试试和大模型Say '搜索一下 你好'", "code_search_tool/code_search_tool.go"},
		{"mcp_agent", "MCP代理 - 学习如何使用MCP代理， 试试和大模型Say '给我用Python在本地写一个冒泡排序'", "mcp_agent/mcp_agent.go"},
	}

	recommendModel := getRecommendModel(models)
	fmt.Printf("%s💡 Recommended Model:%s %s%s%s\n\n", ColorYellow, ColorReset, Bold, recommendModel, ColorReset)

	fmt.Printf("%s📚 Available Exercises:%s\n", ColorGreen, ColorReset)
	for i, exercise := range exercises {
		fmt.Printf("  %s%d.%s %s%s%s\n", ColorCyan, i+1, ColorReset, Bold, exercise.Name, ColorReset)
		fmt.Printf("     📝 %sDescription:%s %s\n", ColorPurple, ColorReset, exercise.Description)
		fmt.Printf("     🚀 %sCommand:%s go run %s --model %s\n\n", ColorBlue, ColorReset, exercise.Path, models[0])
	}
}

func getRecommendModel(models []string) string {
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

func checkOllamaEnvironment() []string {
	fmt.Printf("%s%s═══════════════════════════════════%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("%s%s   Check Ollama Environment%s\n", Bold, ColorCyan, ColorReset)
	fmt.Printf("%s%s═══════════════════════════════════%s\n\n", Bold, ColorBlue, ColorReset)

	// 1. Check if Ollama is installed
	fmt.Printf("%s1.%s Check if Ollama is installed\n", Bold, ColorReset)
	_, err := exec.LookPath("ollama")
	if err != nil {
		fmt.Printf("  %s❌ Ollama is not installed%s\n", ColorRed, ColorReset)
		fmt.Printf("  %s💡 Please install Ollama from https://ollama.ai%s\n", ColorYellow, ColorReset)
		return nil
	}

	fmt.Printf("  %s✅ Ollama is installed%s\n\n", ColorGreen, ColorReset)

	// 2. Check if Ollama is running
	fmt.Printf("%s2.%s Check if Ollama is running\n", Bold, ColorReset)
	cmd := exec.Command("ollama", "list")
	output, err := cmd.Output()
	models := parseOllamaListOutput(string(output))
	if err != nil {
		fmt.Printf("  %s❌ Ollama is not running%s\n", ColorRed, ColorReset)
		fmt.Printf("  %s💡 Suggest: Please start Ollama by running 'ollama serve'%s\n", ColorYellow, ColorReset)
		return nil
	}

	// 3. Check if Ollama has models
	fmt.Printf("%s3.%s Check if Ollama has models\n", Bold, ColorReset)
	if len(models) == 0 {
		fmt.Printf("  %s❌ Ollama does not have any models%s\n", ColorRed, ColorReset)
		fmt.Printf("  %s💡 Please pull a model by running 'ollama pull <model-name>'%s\n", ColorYellow, ColorReset)
		return nil
	}

	fmt.Printf("  %s✅ Ollama has %d model(s)%s\n", ColorGreen, len(models), ColorReset)
	fmt.Printf("%s\n📦 Available Models:%s\n", ColorCyan, ColorReset)
	for i, model := range models {
		fmt.Printf("  %s%d.%s %s%s%s\n", ColorPurple, i+1, ColorReset, Bold, model, ColorReset)
	}
	fmt.Println()
	return models
}

func parseOllamaListOutput(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var models []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "NAME") || strings.Contains(line, "----") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) > 0 {
			modelName := fields[0]
			if modelName != "Name" && !strings.Contains(modelName, "----") {
				models = append(models, fields[0])
			}
		}
	}

	return models
}
