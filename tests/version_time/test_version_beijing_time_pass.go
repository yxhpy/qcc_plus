package main

import (
	"fmt"
	"time"

	"qcc_plus/internal/version"
)

func main() {
	fmt.Println("=== 版本时间转换测试 ===")

	// 测试1: 空值
	version.BuildDate = ""
	result := version.GetFormattedBuildDate()
	fmt.Printf("测试1 - 空值: %s\n", result)
	if result != "未知" {
		fmt.Printf("❌ 失败! 期望 '未知', 得到 '%s'\n", result)
	} else {
		fmt.Println("✅ 通过")
	}

	// 测试2: dev 值
	version.BuildDate = "dev"
	result = version.GetFormattedBuildDate()
	fmt.Printf("\n测试2 - dev值: %s\n", result)
	if result != "开发版本" {
		fmt.Printf("❌ 失败! 期望 '开发版本', 得到 '%s'\n", result)
	} else {
		fmt.Println("✅ 通过")
	}

	// 测试3: RFC3339 UTC 时间
	version.BuildDate = "2025-11-25T10:30:00Z"
	result = version.GetFormattedBuildDate()
	fmt.Printf("\n测试3 - UTC时间: %s\n", result)
	fmt.Printf("原始时间: %s\n", version.BuildDate)
	fmt.Printf("北京时间: %s\n", result)
	// 北京时间应该是 +8 小时
	expected := "2025年11月25日 18时30分00秒"
	if result != expected {
		fmt.Printf("⚠️  期望 '%s', 得到 '%s'\n", expected, result)
	} else {
		fmt.Println("✅ 通过")
	}

	// 测试4: 错误格式
	version.BuildDate = "invalid-date"
	result = version.GetFormattedBuildDate()
	fmt.Printf("\n测试4 - 错误格式: %s\n", result)
	if result != "invalid-date (格式错误)" {
		fmt.Printf("❌ 失败! 期望包含 '(格式错误)', 得到 '%s'\n", result)
	} else {
		fmt.Println("✅ 通过")
	}

	// 测试5: 验证 GetVersionInfo 返回
	version.Version = "v1.0.0-test"
	version.GitCommit = "abc123"
	version.BuildDate = "2025-11-25T02:15:30Z"

	info := version.GetVersionInfo()
	fmt.Printf("\n测试5 - GetVersionInfo 返回:\n")
	fmt.Printf("  Version: %s\n", info.Version)
	fmt.Printf("  GitCommit: %s\n", info.GitCommit)
	fmt.Printf("  BuildDate (UTC): %s\n", info.BuildDate)
	fmt.Printf("  BuildDate (Beijing): %s\n", info.BuildDateBeijing)
	fmt.Printf("  GoVersion: %s\n", info.GoVersion)

	expectedBeijing := "2025年11月25日 10时15分30秒"
	if info.BuildDateBeijing != expectedBeijing {
		fmt.Printf("⚠️  期望 '%s', 得到 '%s'\n", expectedBeijing, info.BuildDateBeijing)
	} else {
		fmt.Println("✅ 北京时间转换正确")
	}

	// 测试6: 当前时间转换
	now := time.Now().UTC()
	version.BuildDate = now.Format(time.RFC3339)
	result = version.GetFormattedBuildDate()
	fmt.Printf("\n测试6 - 当前时间:\n")
	fmt.Printf("  UTC: %s\n", version.BuildDate)
	fmt.Printf("  北京: %s\n", result)
	fmt.Println("✅ 当前时间转换完成")

	fmt.Println("\n=== 测试完成 ===")
}
