package proxy

import (
	"context"
	"log"
	"sync"
	"time"

	"qcc_plus/internal/store"
)

// FailedModelInfo 记录某个节点上某个模型的失败信息。
type FailedModelInfo struct {
	ModelID    string    // 失败的模型 ID
	NodeID     string    // 所属节点 ID
	AccountID  string    // 所属账号 ID
	Error      string    // 最后一次错误信息
	FailedAt   time.Time // 首次失败时间
	LastCheck  time.Time // 最后一次恢复检查时间
	CheckCount      int       // 恢复检查次数
	NonRecoverable  bool      // 是否不可恢复（人工标记）
}

// OfflineDuration 返回模型离线持续时间。
func (f *FailedModelInfo) OfflineDuration() time.Duration {
	return time.Since(f.FailedAt)
}

// ModelRecoveryTracker 跟踪所有节点上失败的模型，用于按模型粒度恢复检查。
// 核心思路：不主动检查所有模型（太贵），只在实际请求失败时记录，
// 然后定时用失败的模型做恢复检查。
// 支持持久化到数据库，重启后自动恢复状态。
type ModelRecoveryTracker struct {
	mu sync.RWMutex
	// nodeModels: nodeID -> modelID -> FailedModelInfo
	nodeModels map[string]map[string]*FailedModelInfo
	store      *store.Store // 可选，nil 时仅内存模式
	logger     *log.Logger
	persistWg  sync.WaitGroup
}

// NewModelRecoveryTracker 创建模型恢复跟踪器。
func NewModelRecoveryTracker() *ModelRecoveryTracker {
	return &ModelRecoveryTracker{
		nodeModels: make(map[string]map[string]*FailedModelInfo),
	}
}

// SetStore 设置持久化存储（在 builder 中调用）。
func (t *ModelRecoveryTracker) SetStore(st *store.Store) {
	t.store = st
}

// SetLogger 设置日志记录器。
func (t *ModelRecoveryTracker) SetLogger(l *log.Logger) {
	t.logger = l
}

// LoadFromStore 从数据库加载失败模型记录，恢复内存状态。
// 应在服务启动时、节点加载完成后调用。
func (t *ModelRecoveryTracker) LoadFromStore() error {
	if t.store == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	records, err := t.store.ListAllFailedModels(ctx)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	loaded := 0
	for _, r := range records {
		models, ok := t.nodeModels[r.NodeID]
		if !ok {
			models = make(map[string]*FailedModelInfo)
			t.nodeModels[r.NodeID] = models
		}
		models[r.ModelID] = &FailedModelInfo{
			ModelID:         r.ModelID,
			NodeID:          r.NodeID,
			AccountID:       r.AccountID,
			Error:           r.Error,
			FailedAt:        r.FailedAt,
			LastCheck:       r.LastCheck,
			CheckCount:      r.CheckCount,
			NonRecoverable:  r.NonRecoverable,
		}
		loaded++
	}

	if loaded > 0 && t.logger != nil {
		t.logger.Printf("[model-recovery] loaded %d failed model records from database", loaded)
	}
	return nil
}

// MarkFailed 标记某个节点上的某个模型为失败状态。
// 如果该模型已经在失败列表中，只更新错误信息，不重置 FailedAt。
// 返回 true 表示是新增的失败记录（首次进入恢复列表），false 表示已存在只是更新。
func (t *ModelRecoveryTracker) MarkFailed(nodeID, modelID, accountID, errMsg string) bool {
	if nodeID == "" || modelID == "" {
		return false
	}
	t.mu.Lock()

	models, ok := t.nodeModels[nodeID]
	if !ok {
		models = make(map[string]*FailedModelInfo)
		t.nodeModels[nodeID] = models
	}

	isNew := false
	if existing, ok := models[modelID]; ok {
		// 已存在，只更新错误信息
		existing.Error = errMsg
	} else {
		models[modelID] = &FailedModelInfo{
			ModelID:   modelID,
			NodeID:    nodeID,
			AccountID: accountID,
			Error:     errMsg,
			FailedAt:  time.Now(),
		}
		isNew = true
	}

	// 复制一份用于异步持久化
	info := *models[modelID]
	t.mu.Unlock()

	// 异步持久化，不阻塞请求路径
	if t.store != nil {
		t.persistWg.Add(1)
		go func() {
			defer t.persistWg.Done()
			t.persistFailed(info)
		}()
	}

	return isNew
}

// MarkRecovered 标记某个节点上的某个模型已恢复。
func (t *ModelRecoveryTracker) MarkRecovered(nodeID, modelID string) {
	if nodeID == "" || modelID == "" {
		return
	}
	t.mu.Lock()

	models, ok := t.nodeModels[nodeID]
	if !ok {
		t.mu.Unlock()
		return
	}
	delete(models, modelID)
	if len(models) == 0 {
		delete(t.nodeModels, nodeID)
	}
	t.mu.Unlock()

	// 异步删除持久化记录
	if t.store != nil {
		t.persistWg.Add(1)
		go func() {
			defer t.persistWg.Done()
			t.persistRecovered(nodeID, modelID)
		}()
	}
}

// MarkNodeRecovered 清除某个节点上所有失败的模型（节点整体恢复时调用）。
func (t *ModelRecoveryTracker) MarkNodeRecovered(nodeID string) {
	if nodeID == "" {
		return
	}
	t.mu.Lock()
	delete(t.nodeModels, nodeID)
	t.mu.Unlock()

	// 异步删除持久化记录
	if t.store != nil {
		t.persistWg.Add(1)
		go func() {
			defer t.persistWg.Done()
			t.persistNodeRecovered(nodeID)
		}()
	}
}

// GetFailedModels 获取某个节点上所有失败的模型。
func (t *ModelRecoveryTracker) GetFailedModels(nodeID string) []*FailedModelInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	models, ok := t.nodeModels[nodeID]
	if !ok {
		return nil
	}

	result := make([]*FailedModelInfo, 0, len(models))
	for _, info := range models {
		cp := *info
		result = append(result, &cp)
	}
	return result
}

// GetAllFailed 获取所有节点上所有失败的模型，用于 API 展示。
func (t *ModelRecoveryTracker) GetAllFailed() []*FailedModelInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*FailedModelInfo
	for _, models := range t.nodeModels {
		for _, info := range models {
			cp := *info
			result = append(result, &cp)
		}
	}
	return result
}

// GetAllFailedByAccount 获取指定账号下所有失败的模型。
func (t *ModelRecoveryTracker) GetAllFailedByAccount(accountID string) []*FailedModelInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*FailedModelInfo
	for _, models := range t.nodeModels {
		for _, info := range models {
			if info.AccountID == accountID {
				cp := *info
				result = append(result, &cp)
			}
		}
	}
	return result
}

// GetPendingRecoveryChecks 获取需要进行恢复检查的模型列表。
// 返回每个节点需要检查的模型（去重），同时更新 LastCheck 和 CheckCount。
func (t *ModelRecoveryTracker) GetPendingRecoveryChecks() map[string][]string {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string][]string)
	for nodeID, models := range t.nodeModels {
		for modelID, info := range models {
			if info.NonRecoverable {
				continue
			}
			info.LastCheck = time.Now()
			info.CheckCount++
			result[nodeID] = append(result[nodeID], modelID)
		}
	}
	return result
}
// Count 返回当前跟踪的失败模型总数。
func (t *ModelRecoveryTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, models := range t.nodeModels {
		count += len(models)
	}
	return count
}

// IsModelFailed 检查某个节点上的某个模型是否处于失败状态。
func (t *ModelRecoveryTracker) IsModelFailed(nodeID, modelID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	models, ok := t.nodeModels[nodeID]
	if !ok {
		return false
	}
	_, failed := models[modelID]
	return failed
}

// --- 持久化辅助方法 ---

func (t *ModelRecoveryTracker) persistFailed(info FailedModelInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := t.store.UpsertFailedModel(ctx, store.FailedModelRecord{
		NodeID:         info.NodeID,
		ModelID:        info.ModelID,
		AccountID:      info.AccountID,
		Error:          info.Error,
		FailedAt:       info.FailedAt,
		LastCheck:      info.LastCheck,
		CheckCount:     info.CheckCount,
		NonRecoverable: info.NonRecoverable,
	})
	if err != nil && t.logger != nil {
		t.logger.Printf("[model-recovery] persist failed model error: %v", err)
	}
}

func (t *ModelRecoveryTracker) persistRecovered(nodeID, modelID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := t.store.DeleteFailedModel(ctx, nodeID, modelID)
	if err != nil && t.logger != nil {
		t.logger.Printf("[model-recovery] persist recovered model error: %v", err)
	}
}

func (t *ModelRecoveryTracker) persistNodeRecovered(nodeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := t.store.DeleteFailedModelsByNode(ctx, nodeID)
	if err != nil && t.logger != nil {
		t.logger.Printf("[model-recovery] persist node recovered error: %v", err)
	}
}

// SetNonRecoverable 设置某个失败模型是否不可恢复。
func (t *ModelRecoveryTracker) SetNonRecoverable(nodeID, modelID string, nonRecoverable bool) {
	if nodeID == "" || modelID == "" {
		return
	}
	t.mu.Lock()
	models, ok := t.nodeModels[nodeID]
	if !ok {
		t.mu.Unlock()
		return
	}
	info, ok := models[modelID]
	if !ok {
		t.mu.Unlock()
		return
	}
	info.NonRecoverable = nonRecoverable
	snapshot := *info
	t.mu.Unlock()

	if t.store != nil {
		t.persistWg.Add(1)
		go func() {
			defer t.persistWg.Done()
			t.persistFailed(snapshot)
		}()
	}
}

// Wait waits for queued persistence work to finish.
func (t *ModelRecoveryTracker) Wait() {
	if t == nil {
		return
	}
	t.persistWg.Wait()
}
