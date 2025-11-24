# Claude Code CLI 验证

该验证用例在 Docker 容器中运行 Claude Code CLI（无头模式），用于健康检查。

## 准备环境变量
```bash
export ANTHROPIC_API_KEY="88_820f837e55c1d16735a79fa4c7cfdfeeab135f965e65b1b930e8f1543998caae"
export ANTHROPIC_AUTH_TOKEN="88_820f837e55c1d16735a79fa4c7cfdfeeab135f965e65b1b930e8f1543998caae"
export ANTHROPIC_BASE_URL="https://www.88code.org/api"
# 可选自定义提示词，未设置则默认“健康检查：请回复 OK”
export CLAUDE_PROMPT="请输出 OK"
```

## 运行验证
在仓库根目录执行：
```bash
go run verify/claude_code_cli/verify_cli.go
```

程序步骤：
- 使用 `Dockerfile.verify` 构建镜像（包含 Node.js 与 `@anthropic-ai/claude-code`）。
- 通过 `docker run` 注入上述环境变量，执行 `claude -p <prompt>`。
- 捕获并打印 CLI 输出，输出非空即视为成功。

## 成功后标记
验证通过后，可按需将文件重命名以标记：
```bash
mv verify/claude_code_cli/Dockerfile.verify verify/claude_code_cli/Dockerfile.verify_pass
mv verify/claude_code_cli/verify_cli.go verify/claude_code_cli/verify_cli_pass.go
```
如需再次运行，请将文件名改回原名或更新 `go run` 路径。
