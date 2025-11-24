package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"qcc_plus/internal/store"
)

// 验证 UpsertNode 是否正确保留统计数据
func main() {
	// 使用内存数据库进行测试
	dsn := "root@tcp(localhost:3307)/qcc_proxy?parseTime=true"

	st, err := store.Open(dsn)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	nodeID := "test-node-" + fmt.Sprint(time.Now().Unix())

	// 第一次插入：创建节点并设置统计数据
	initialRecord := store.NodeRecord{
		ID:                nodeID,
		Name:              "Test Node",
		BaseURL:           "https://api.example.com",
		APIKey:            "sk-test-key-1",
		HealthCheckMethod: "api",
		AccountID:         store.DefaultAccountID,
		Weight:            10,
		CreatedAt:         time.Now(),
		Requests:          100,
		FailCount:         5,
		FailStreak:        2,
		TotalBytes:        10000,
		TotalInput:        5000,
		TotalOutput:       3000,
		StreamDurMs:       500,
		FirstByteMs:       50,
		LastPingMs:        10,
	}

	if err := st.UpsertNode(ctx, initialRecord); err != nil {
		log.Fatalf("Failed to insert initial record: %v", err)
	}
	fmt.Printf("✅ 第一次插入成功：Requests=%d, FailCount=%d, TotalInput=%d, TotalOutput=%d\n",
		initialRecord.Requests, initialRecord.FailCount, initialRecord.TotalInput, initialRecord.TotalOutput)

	// 第二次更新：只更新配置字段
	updateRecord := store.NodeRecord{
		ID:                nodeID,
		Name:              "Updated Node Name",
		BaseURL:           "https://api.updated.com",
		APIKey:            "sk-test-key-2",
		HealthCheckMethod: "api",
		AccountID:         store.DefaultAccountID,
		Weight:            20,
		CreatedAt:         time.Now(), // 新的时间
		// 统计字段都是零值
		Requests:    0,
		FailCount:   0,
		FailStreak:  0,
		TotalBytes:  0,
		TotalInput:  0,
		TotalOutput: 0,
		StreamDurMs: 0,
		FirstByteMs: 0,
		LastPingMs:  0,
	}

	if err := st.UpsertNode(ctx, updateRecord); err != nil {
		log.Fatalf("Failed to update record: %v", err)
	}
	fmt.Printf("✅ 第二次更新成功（传入零值统计）\n")

	// 重新加载并验证
	records, _, _, err := st.LoadAllByAccount(ctx, store.DefaultAccountID)
	if err != nil {
		log.Fatalf("Failed to load records: %v", err)
	}

	var found *store.NodeRecord
	for i := range records {
		if records[i].ID == nodeID {
			found = &records[i]
			break
		}
	}

	if found == nil {
		log.Fatalf("❌ 节点未找到")
	}

	// 验证配置字段已更新
	if found.Name != "Updated Node Name" {
		log.Fatalf("❌ Name 未更新: got=%s, want=Updated Node Name", found.Name)
	}
	if found.BaseURL != "https://api.updated.com" {
		log.Fatalf("❌ BaseURL 未更新: got=%s, want=https://api.updated.com", found.BaseURL)
	}
	if found.APIKey != "sk-test-key-2" {
		log.Fatalf("❌ APIKey 未更新: got=%s, want=sk-test-key-2", found.APIKey)
	}
	if found.Weight != 20 {
		log.Fatalf("❌ Weight 未更新: got=%d, want=20", found.Weight)
	}
	fmt.Printf("✅ 配置字段已正确更新：Name=%s, Weight=%d\n", found.Name, found.Weight)

	// 验证统计字段保持不变
	if found.Requests != 100 {
		log.Fatalf("❌ Requests 被重置: got=%d, want=100", found.Requests)
	}
	if found.FailCount != 5 {
		log.Fatalf("❌ FailCount 被重置: got=%d, want=5", found.FailCount)
	}
	if found.TotalInput != 5000 {
		log.Fatalf("❌ TotalInput 被重置: got=%d, want=5000", found.TotalInput)
	}
	if found.TotalOutput != 3000 {
		log.Fatalf("❌ TotalOutput 被重置: got=%d, want=3000", found.TotalOutput)
	}
	fmt.Printf("✅ 统计字段保持不变：Requests=%d, FailCount=%d, TotalInput=%d, TotalOutput=%d\n",
		found.Requests, found.FailCount, found.TotalInput, found.TotalOutput)

	// 清理测试数据
	if err := st.DeleteNode(ctx, nodeID); err != nil {
		log.Fatalf("Failed to delete test node: %v", err)
	}

	fmt.Println("\n🎉 持久化验证通过！统计数据在更新配置时被正确保留。")
}
