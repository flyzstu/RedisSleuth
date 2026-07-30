# RedisSleuth — Redis Cluster 性能诊断工具

[![CI](https://github.com/flyzstu/RedisSleuth/actions/workflows/ci.yml/badge.svg)](https://github.com/flyzstu/RedisSleuth/actions/workflows/ci.yml)
[![Security](https://github.com/flyzstu/RedisSleuth/actions/workflows/security.yml/badge.svg)](https://github.com/flyzstu/RedisSleuth/actions/workflows/security.yml)

RedisSleuth 是面向 Redis Cluster 的只读、低侵入性能诊断 CLI，用于排查 CPU、内存、槽位、
Key 和客户端连接问题。使用 Go 1.23+，支持 Linux 与 Windows 构建，兼容 Redis 5.0.x。

> Repository description: Read-only, low-intrusion Redis Cluster CPU, memory,
> slot, key, and client diagnostics for Redis 5.0+.

生产环境的分步排障流程、影响评估、停止条件和事件记录模板见
[生产排障 Runbook](docs/runbook.md)。

## 下载

预编译的 Linux/Windows amd64、arm64 包和 SHA-256 校验文件可从
[GitHub Releases](https://github.com/flyzstu/RedisSleuth/releases) 下载。

## 构建与运行

```bash
go build -o redissleuth .
export REDIS_PASSWORD='your-password'
./redissleuth diagnose \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD \
  --duration 30s \
  --output table
```

也可以使用配置文件：

```bash
./redissleuth diagnose --config examples/config.yaml
./redissleuth cpu --config examples/config.yaml --duration 10s --output json
```

CLI 显式参数优先于 YAML。可用命令为 `topology`、`diagnose`、`cpu`、`memory`、`slots` 和
`clients`。`--command-timeout` 控制每个 Redis 请求的超时；`--scan-count`、`--sample-size`、
`--scan-rate` 和 `--top` 控制抽样。Key 默认脱敏，只有明确传入 `--show-full-key` 才显示原文。

JSON 固定提供 `metadata`、`cluster`、`nodes`、`findings` 和 `recommendations`，并按命令增加
CPU、内存、槽位、客户端和 Key 样本字段。Table 输出为中文。

## 安全与 Redis 5.0.x 兼容

- 工具不包含 Redis 写操作，不使用 `KEYS *`，也不默认启用或提供 `MONITOR`。
- 所有连接和命令都有超时，SCAN 有样本上限与速率限制，并优先访问 replica。
- Redis 5 没有 ACL；配置 `username: default` 时会使用兼容的单参数 AUTH。非 default 用户名只适用于 Redis 6+。
- Redis 5 可能不提供 `allocator_frag_ratio`、`allocator_rss_ratio`、`mem_clients_normal` 等较新 INFO 字段；报告会保留零值/未知值，而不会把缺失字段判成异常。
- TLS 取决于服务端构建和部署方式；官方 Redis 5 默认不原生提供 TLS。
- `--tls-insecure` 仅用于受控排障环境，会关闭证书校验；生产环境不应启用。

## MVP 限制

- Redis 默认不会长期记录完整的“客户端 IP—Key”对应关系。
- MVP 只能利用 SLOWLOG、CLIENT LIST 和同一时间窗口做有限关联；没有明确证据时不会声称某客户端访问了某个 Key。
- 不默认启用 MONITOR，当前 CLI 也不执行 MONITOR。
- Key 分析基于 SCAN 抽样，不代表全量数据；大 Key 数量和无 TTL 比例都是样本结论。
- 槽位负载根据节点指标和 Key 抽样推断，不遍历全部 16384 个槽位。
- Redis CPU 百分比由累计 CPU 时间差值计算，表示采样周期内 Redis 进程消耗的单核 CPU 百分比。
- 工具不替代 Redis Exporter、Prometheus 与系统级 CPU、RSS、Swap、网络和磁盘监控。
- 当前仅支持 Redis Cluster，不支持 Sentinel 和单机模式。
- “连接风暴”在 MVP 中主要基于同一 IP 当前连接数阈值判断；若要准确识别突增需接入历史监控。
- replica 不可用时会回退到 master 抽样，因此生产环境应设置保守的 scan rate 和 sample size。

## 开发检查

```bash
go test ./...
go vet ./...
go build ./...
```

GitHub Actions 会在 push、Pull Request 和每周计划任务中执行：

- Go 测试、vet、构建和模块校验；
- `govulncheck` 可达依赖漏洞扫描；
- `gosec` Go 代码安全风险扫描；
- GitHub CodeQL 代码扫描。

推送符合 `v*.*.*` 格式的版本标签时，Release 工作流会验证项目、交叉编译 Linux/Windows
amd64 和 arm64、生成 SHA-256 校验文件，并自动创建 GitHub Release 和上传产物。

详细设计见 [docs/design.md](docs/design.md)，完整配置见
[examples/config.yaml](examples/config.yaml)。
