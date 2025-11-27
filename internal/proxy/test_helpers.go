package proxy

import "fmt"

// TestSetCLIRunner 覆盖 CLI 健康检查执行器（仅用于测试）。
func (p *Server) TestSetCLIRunner(r CliRunner) {
	if p == nil {
		return
	}
	p.cliRunner = r
}

// TestAddNode 便于测试时快速添加节点。
func (p *Server) TestAddNode(accountID, name, baseURL, apiKey, healthMethod string, weight int) (*Node, error) {
	acc := p.getAccountByID(accountID)
	if acc == nil {
		return nil, fmt.Errorf("account %s not found", accountID)
	}
	return p.addNodeWithMethod(acc, name, baseURL, apiKey, weight, healthMethod)
}

// TestAccount 获取指定账号（测试辅助）。
func (p *Server) TestAccount(id string) *Account {
	return p.getAccountByID(id)
}

// TestCheckNodeHealth 触发一次健康检查（测试辅助）。
func (p *Server) TestCheckNodeHealth(id string) {
	p.mu.RLock()
	acc := p.nodeAccount[id]
	p.mu.RUnlock()
	p.checkNodeHealth(acc, id, CheckSourceScheduled)
}
