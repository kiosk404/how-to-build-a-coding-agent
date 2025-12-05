package main

import (
	"encoding/json"
	"fmt"
)

func (a *Agent) InputUnLock() {
	a.inputLock.Lock()
	defer a.inputLock.Unlock()
	a.isProcessing = false
}

func (a *Agent) InputLock() {
	a.inputLock.Lock()
	defer a.inputLock.Unlock()
	a.isProcessing = true
}

// formatToolResult 将工具返回结果格式化为字符串
func formatToolResult(result interface{}) string {
	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		// 尝试 JSON 序列化
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

// truncateString 截断字符串用于显示
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
