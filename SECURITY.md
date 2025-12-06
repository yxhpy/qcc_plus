# 安全策略

## 支持的版本

目前支持安全更新的版本：

| 版本 | 支持状态 |
| --- | --- |
| v1.0.x | ✅ 支持 |
| < v1.0.0 | ❌ 不支持 |

## 报告安全漏洞

如果你发现了安全漏洞，**请不要公开发布 Issue**。

### 报告流程

1. **发送邮件**：将漏洞详情发送至项目维护者
   - 邮件主题：`[Security] qcc_plus 安全漏洞报告`

2. **包含信息**：
   - 漏洞描述
   - 影响版本
   - 复现步骤
   - 可能的影响范围
   - 建议的修复方案（如果有）

3. **等待响应**：
   - 我们会在 48 小时内确认收到报告
   - 在 7 天内评估漏洞并制定修复计划
   - 修复完成后会通知你

### 漏洞处理流程

1. **确认**：验证漏洞的存在和影响范围
2. **修复**：开发补丁并进行测试
3. **发布**：发布包含修复的新版本
4. **公告**：在修复版本发布后公开漏洞详情

## 安全最佳实践

### 部署安全

1. **修改默认凭证**
   ```bash
   # ⚠️ 生产环境必须修改
   export ADMIN_API_KEY=your-secure-random-key
   export DEFAULT_PROXY_API_KEY=your-proxy-key
   ```

2. **使用强密码**
   - 管理员密码：至少 16 位，包含大小写字母、数字和特殊字符
   - 定期轮换密码（建议每 90 天）

3. **限制网络访问**
   ```bash
   # 仅监听本地
   export LISTEN_ADDR=127.0.0.1:8000

   # 使用反向代理（Nginx/Caddy）
   # 配置 IP 白名单
   ```

4. **启用 HTTPS**
   - 使用 Let's Encrypt 证书
   - 配置 TLS 1.2+ 和强加密套件
   - 启用 HSTS

5. **API Token 安全**
   ```bash
   # 不要在环境变量中使用真实 Token（开发环境除外）
   # 使用密钥管理服务（如 HashiCorp Vault）
   # 定期轮换 Token
   ```

### 数据库安全

1. **访问控制**
   - 使用专用数据库用户
   - 授予最小权限（SELECT, INSERT, UPDATE, DELETE）
   - 禁用 DROP, ALTER 权限

2. **连接安全**
   ```bash
   # 使用 TLS 连接
   export PROXY_MYSQL_DSN="user:pass@tcp(host:3306)/db?parseTime=true&tls=true"

   # 限制来源 IP
   # 在 MySQL 配置中设置 bind-address
   ```

3. **备份**
   - 定期备份数据库
   - 加密备份文件
   - 测试恢复流程

### 运行时安全

1. **容器安全**
   ```yaml
   # docker-compose.yml
   services:
     proxy:
       security_opt:
         - no-new-privileges:true
       read_only: true
       tmpfs:
         - /tmp
       user: "1000:1000"  # 非 root 用户
   ```

2. **日志安全**
   - 不要记录敏感信息（Token、密码）
   - 使用日志轮转
   - 限制日志访问权限

3. **更新**
   - 及时更新到最新版本
   - 订阅安全公告
   - 定期检查依赖漏洞

### Cloudflare Tunnel 安全

1. **API Token 权限**
   - 仅授予必需权限（Tunnel Edit + DNS Edit）
   - 为不同环境使用不同 Token
   - 记录 Token 使用情况

2. **访问控制**
   - 在 Cloudflare Dashboard 配置访问策略
   - 启用 Cloudflare Access（Zero Trust）
   - 设置 IP 白名单

## 已知安全问题

### 默认凭证（已解决）

**问题**：早期版本使用硬编码的默认凭证

**影响版本**：< v1.0.0

**修复版本**：v1.0.0+

**解决方案**：
- 启动时强制提示修改默认凭证
- 文档中明确警告安全风险

### Session Token 安全

**当前状态**：使用 HttpOnly Cookie 存储 session token

**安全措施**：
- Cookie 设置 HttpOnly 和 Secure 标志
- 24 小时过期
- 服务端验证

**建议改进**（未来版本）：
- 实现 Token 刷新机制
- 支持会话过期通知
- 添加并发会话限制

## 安全审计

项目定期进行安全审计：

- **代码审计**：每个 PR 都会进行代码审查
- **依赖审计**：使用 `go mod` 检查依赖漏洞
- **容器扫描**：Docker 镜像使用 Trivy 扫描

## 安全配置检查清单

部署前检查：

- [ ] 已修改所有默认凭证
- [ ] 管理员密码符合强度要求
- [ ] 已配置 HTTPS/TLS
- [ ] 数据库连接已加密
- [ ] 限制了网络访问范围
- [ ] 日志不包含敏感信息
- [ ] 容器以非 root 用户运行
- [ ] 已启用防火墙规则
- [ ] 已设置监控和告警
- [ ] 定期备份已配置

## 联系方式

- **安全问题**：通过 GitHub Security Advisories 私密报告
- **一般问题**：在 [GitHub Issues](https://github.com/yxhpy/qcc_plus/issues) 提问

## 致谢

感谢所有报告安全问题的贡献者！

---

最后更新：2025-12-06
