# 推送后宕机问题诊断指南

## 可能原因

### 1. 数据库迁移失败 ⚠️ **高概率**
新增的 `health_check_model` 字段可能导致迁移失败

**检查方法**：
```bash
ssh root@43.156.77.170 "docker logs qcc_test-proxy-1 2>&1 | grep -i 'migration\|health_check_model\|error'"
```

**解决方案**：
- 检查 `internal/store/migration.go` 中的 SQL 语法
- 确认数据库中表结构是否正确
- 手动执行 SQL 迁移

### 2. 启动时健康检查阻塞 ⚠️ **中等概率**
启动时立即执行 `checkAllNodes()` 可能造成负载峰值

**问题位置**：`internal/proxy/health_scheduler.go:80`

**检查方法**：
```bash
ssh root@43.156.77.170 "docker logs qcc_test-proxy-1 2>&1 | grep -i 'HealthScheduler'"
```

**解决方案**：延迟首次全量健康检查

### 3. Panic 未捕获 ⚠️ **中等概率**
启动时可能有 nil 指针或其他 panic

**检查方法**：
```bash
ssh root@43.156.77.170 "docker logs qcc_test-proxy-1 2>&1 | grep -i 'panic'"
```

### 4. 健康检查超时 ⚠️ **低概率**
部署脚本等待 60 秒，如果服务启动慢可能超时

**检查方法**：
```bash
# 查看最近的 GitHub Actions 运行日志
```

### 5. 端口占用 ⚠️ **低概率**
8001 端口可能被旧容器占用

**检查方法**：
```bash
ssh root@43.156.77.170 "docker ps -a | grep 8001"
ssh root@43.156.77.170 "lsof -i :8001"
```

## 诊断步骤

### 步骤 1：检查容器状态
```bash
ssh root@43.156.77.170 "docker ps -a | grep qcc_test"
```

### 步骤 2：检查最新日志（最重要！）
```bash
ssh root@43.156.77.170 "docker logs --tail 100 qcc_test-proxy-1 2>&1"
```

### 步骤 3：检查数据库
```bash
ssh root@43.156.77.170 "docker exec qcc_test-mysql-1 mysql -uroot -p123456 qcc_plus -e 'DESCRIBE nodes'"
```

### 步骤 4：检查资源使用
```bash
ssh root@43.156.77.170 "docker stats --no-stream qcc_test-proxy-1"
```

### 步骤 5：手动重启容器
```bash
ssh root@43.156.77.170 "cd /opt/qcc_plus && docker compose -p qcc_test -f docker-compose.test.yml restart proxy"
```

## 快速修复方案

### 如果是数据库迁移失败
```bash
# 1. 手动添加字段
ssh root@43.156.77.170 "docker exec qcc_test-mysql-1 mysql -uroot -p123456 qcc_plus -e \"ALTER TABLE nodes ADD COLUMN health_check_model VARCHAR(128) DEFAULT 'claude-haiku-4-5-20251001' AFTER health_check_method\""

# 2. 重启容器
ssh root@43.156.77.170 "cd /opt/qcc_plus && docker compose -p qcc_test -f docker-compose.test.yml restart proxy"
```

### 如果是启动阻塞
延迟首次健康检查，修改 `health_scheduler.go:79-80`：
```go
// 延迟执行，避免启动时负载峰值
time.AfterFunc(30*time.Second, h.checkAllNodes)
```

### 如果是 Panic
添加全局 panic 恢复，修改 `cmd/cccli/main.go`：
```go
defer func() {
    if r := recover(); r != nil {
        log.Printf("FATAL PANIC: %v\n%s", r, debug.Stack())
    }
}()
```

## 预防措施

1. **本地测试**：
   ```bash
   go run ./cmd/cccli proxy
   # 确保本地能正常启动
   ```

2. **数据库迁移测试**：
   ```bash
   # 启动本地 MySQL
   docker run -d --name mysql-test -e MYSQL_ROOT_PASSWORD=123456 -e MYSQL_DATABASE=qcc_plus -p 3307:3306 mysql:8

   # 测试迁移
   PROXY_MYSQL_DSN="root:123456@tcp(localhost:3307)/qcc_plus" go run ./cmd/cccli proxy
   ```

3. **逐步部署**：
   - 先部署到测试环境
   - 观察 24 小时
   - 确认稳定后再部署到生产

## 紧急回滚

如果服务完全不可用：
```bash
ssh root@43.156.77.170 "cd /opt/qcc_plus && git checkout test && git reset --hard HEAD~1 && ./scripts/deploy-server.sh test"
```

## 联系方式
如果无法解决，请提供：
1. `docker logs qcc_test-proxy-1` 完整日志
2. GitHub Actions 运行日志
3. 服务器资源使用情况（`docker stats`）
