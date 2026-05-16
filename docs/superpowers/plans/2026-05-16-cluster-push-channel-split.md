# Cluster Push Channel Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将集群推送从单一 Redis channel 拆成 `users`、`group`、`broadcast` 三类 channel，同时保持现有 RPC 和消息体不变。

**Architecture:** 保留 `PushEvent` 和全实例订阅模型，只把 channel 选择从单通道改为按目标类型路由。配置层继续支持现有 `redis.push_channel` 作为基础前缀，并派生三类默认 channel，避免直接破坏现有部署。

**Tech Stack:** Go 1.26、Viper/PFlag、Redis Pub/Sub、Google Wire、signalg

---

### Task 1: 配置层支持三类推送 channel

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] 增加 `users/group/broadcast` 三类 channel 配置字段与归一化逻辑。
- [ ] 保留 `redis.push_channel` 作为基础前缀，未显式配置时派生默认 channel。
- [ ] 补充默认值、YAML、环境变量与 flag 的测试覆盖。

### Task 2: cluster 层按目标类型选择 channel

**Files:**
- Modify: `internal/cluster/push.go`
- Test: `internal/cluster/push_test.go`

- [ ] 让 `Publisher.Publish` 根据 `PushEvent.Target` 选择目标 channel。
- [ ] 让 `Subscriber` 同时订阅三类 channel，并对重复 channel 做去重。
- [ ] 补充 users/group/broadcast 的发布与订阅测试。

### Task 3: 应用装配与日志对齐

**Files:**
- Modify: `internal/app/app.go`

- [ ] 更新订阅启动日志，输出实际订阅的 channel 集合。
- [ ] 保持 Wire 装配和 RPC 层接口不变，确保改动面最小。

### Task 4: 回归验证

**Files:**
- Test: `internal/config/config_test.go`
- Test: `internal/cluster/push_test.go`
- Test: `internal/rpc/service_test.go`

- [ ] 运行配置与 cluster 定向测试，确认红绿过程完整。
- [ ] 运行相关包回归测试，确认没有引入接口回归。
