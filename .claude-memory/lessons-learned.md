# 经验教训

## 说明

本文件是项目记忆系统的一部分，用于记录开发过程中的经验教训。

**主要经验教训文档**: `docs/claude/lessons-learned.md`

项目已有完善的踩坑记录系统，详见：
- [docs/claude/lessons-learned.md](../docs/claude/lessons-learned.md) - 详细的踩坑记录

## 升级相关经验

### 经验 1: 项目升级为自驱动 AI 项目
**日期**: 2026-02-02
**场景**: 将传统项目升级为 AI 项目
**教训**:
- ✅ 保留所有现有代码和配置
- ✅ 增量添加新功能，不破坏现有结构
- ✅ 充分利用现有文档和规范
- ✅ 建立模块注册表防止重复造轮子

### 经验 2: 文档系统整合
**日期**: 2026-02-02
**场景**: 整合现有文档系统
**教训**:
- ✅ 不重复创建已有文档
- ✅ 建立文档索引和引用关系
- ✅ 保持文档结构清晰
- ✅ 使用脚本自动更新文档

## 快速添加记录

遇到问题解决后，可以添加到以下任一位置：
1. `docs/claude/lessons-learned.md` - 详细的踩坑记录（推荐）
2. 本文件 - 简要的经验总结

## 相关文档

- [docs/claude/lessons-learned.md](../docs/claude/lessons-learned.md) - 详细踩坑记录
- [CLAUDE.md](../CLAUDE.md) - 项目记忆文件
- [docs/claude/debug-playbook.md](../docs/claude/debug-playbook.md) - 调试排查手册
