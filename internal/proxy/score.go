package proxy

// CalculateScore 根据窗口指标计算节点得分（越低越优先）。
// 当窗口为空或缺失时，退回使用节点权重。
func CalculateScore(node *Node, alphaErr, betaLat float64) float64 {
	if node == nil {
		return 0
	}
	baseWeight := float64(node.Weight)
	if node.Window == nil {
		return baseWeight
	}

	successRate := node.Window.SuccessRate()
	p95 := node.Window.P95Latency()

	return baseWeight + alphaErr*(1-successRate) + betaLat*(p95/1000.0)
}
