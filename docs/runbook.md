# RedisSleuth 生产环境排障 Runbook

本文用于在 Redis Cluster 出现 CPU 高、内存高、连接异常、复制异常或槽位倾斜时，指导值班人员
以只读、低侵入方式收集证据和形成初步判断。

RedisSleuth 不能替代 Redis Exporter、Prometheus、主机监控和业务链路追踪。任何结论都应结合
历史基线、主机指标和应用日志验证。

## 1. 安全边界

RedisSleuth 遵循以下边界：

- 不执行 Redis 数据写操作；
- 不使用 `KEYS *`；
- 不执行 `MONITOR`；
- 所有 Redis 请求都有超时；
- Key 分析使用有数量和速率限制的 `SCAN`；
- 优先在 replica 上抽样，replica 不可用时才回退到 master；
- Key 默认脱敏；
- 不会在缺少直接证据时断言某客户端访问了某个 Key。

排障人员仍应遵守：

- 使用只读或最小权限账号；
- 通过环境变量传递密码；
- 报告文件保存到受控目录；
- 未经批准不要使用 `--show-full-key`；
- 业务高峰期使用保守的抽样参数；
- 发现延迟上升或复制延迟扩大时立即停止 Key 抽样。

## 2. 影响等级

| 命令 | 影响等级 | 主要请求 | 注意事项 |
|---|---:|---|---|
| `topology` | 很低 | CLUSTER、INFO、PING | 可优先执行 |
| `cpu` | 低 | 周期性 INFO、SLOWLOG | 采样时间越长，请求次数越多 |
| `memory` | 低 | 周期性 INFO | 不遍历 Key |
| `clients` | 低到中 | CLIENT LIST | 数万连接时响应可能较大 |
| `slots` | 中 | INFO、SCAN、逐 Key 检查 | 使用保守抽样参数 |
| `diagnose` | 中 | 上述检查的组合 | Key 抽样与指标采集并行 |

每个 Key 样本通常产生 `TYPE`、`PTTL`、`MEMORY USAGE` 和一个元素数量命令，因此
`--scan-rate 100` 可能对应每秒约 300～400 个轻量只读命令，而不是 100 条命令。

推荐参数：

| 场景 | scan-count | sample-size | scan-rate |
|---|---:|---:|---:|
| 高峰期首次排查 | 20 | 100 | 20 |
| 常规生产排查 | 50 | 200 | 50 |
| 低峰期深入抽样 | 100 | 1000 | 200 |

默认值上限较高，繁忙生产集群应显式传入保守参数。

## 3. 排障前检查

确认工具版本：

```bash
redissleuth --version
```

设置密码环境变量：

```bash
export REDIS_PASSWORD='replace-me'
```

Windows PowerShell：

```powershell
$env:REDIS_PASSWORD = "replace-me"
```

准备受控的报告目录，并记录：

- 告警开始时间；
- 告警节点；
- 当前业务峰值或发布事件；
- Redis Exporter/Prometheus 面板链接；
- 应用负责人和 Redis 值班人员。

通用连接参数：

```text
--addr 10.0.1.11:6379
--username default
--password-env REDIS_PASSWORD
--connect-timeout 3s
--command-timeout 2s
```

Redis 5 没有 ACL。`--username default` 会使用兼容 Redis 5 的单参数 AUTH。

## 4. 标准排障流程

### 步骤一：确认拓扑和复制关系

```bash
redissleuth topology \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD
```

检查：

- `cluster_state` 是否为 `ok`；
- 节点数量是否符合预期；
- master 槽位是否覆盖完整；
- 是否存在 `fail`、`noaddr` 或 `disconnected`；
- replica 的 master ID 是否正确；
- replica 主从链路是否为 `up`。

若集群状态异常，先处理网络、节点存活和复制问题，不要立即扩大 Key 抽样。

### 步骤二：采样 CPU

```bash
redissleuth cpu \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD \
  --duration 30s \
  --interval 5s
```

CPU 百分比计算口径：

```text
(used_cpu_sys + used_cpu_user 的累计差值) / 实际采样秒数 × 100
```

该值表示 Redis 进程的单核等效 CPU 百分比，不是整台主机 CPU。检查：

- 是否只有一个 master CPU 明显较高；
- CPU 高是否伴随 OPS 高；
- 哪个命令的调用增量或耗时增量最高；
- 是否产生新慢日志；
- 是否发生 BGSAVE、AOF rewrite 或全量同步。

同时使用系统监控确认：

```bash
pidstat -p <redis-pid> 1
vmstat 1
iostat -xz 1
```

### 步骤三：检查内存

```bash
redissleuth memory \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD \
  --duration 30s
```

检查：

- 利用率分母是 `maxmemory` 还是 `total_system_memory`；
- RSS 是否明显高于 `used_memory`；
- `mem_fragmentation_ratio` 是否过高或小于 1；
- `evicted_keys` 是否在采样期间增长；
- 客户端缓冲区和复制 backlog 是否偏高；
- 三个 master 的内存是否明显不均衡；
- master 和 replica 是否存在明显内存差异。

碎片率小于 1 时检查 Swap：

```bash
free -h
swapon --show
vmstat 1
```

### 步骤四：检查客户端

```bash
redissleuth clients \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD
```

检查：

- 单一 IP 的连接总数；
- 活跃与空闲连接比例；
- 当前命令分布；
- 输入、输出缓冲区；
- 是否存在连接集中或疑似连接风暴。

`CLIENT LIST` 只反映当前连接状态，不能证明某个 IP 曾访问某个具体 Key。

### 步骤五：保守执行槽位和 Key 抽样

```bash
redissleuth slots \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD \
  --scan-count 20 \
  --sample-size 100 \
  --scan-rate 20 \
  --top 20
```

检查：

- master 的槽位数量、Key 数、内存和 OPS 是否倾斜；
- 抽样热点槽位；
- 大 Hash、List、Set、ZSet 或 Stream；
- 大量无 TTL Key；
- Hash Tag 是否导致业务数据集中。

如果首次抽样没有明确结论，且 Redis 延迟、CPU 和复制状态稳定，可在低峰期逐级增加
`sample-size` 和 `scan-rate`。不要一次跳到大规模扫描。

### 步骤六：生成统一诊断报告

```bash
redissleuth diagnose \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD \
  --duration 30s \
  --scan-count 20 \
  --sample-size 200 \
  --scan-rate 50 \
  --top 20 \
  --output json > diagnose.json
```

如果只需要指标和客户端证据，不希望抽样 Key：

```bash
redissleuth diagnose \
  --addr 10.0.1.11:6379 \
  --username default \
  --password-env REDIS_PASSWORD \
  --duration 30s \
  --sample-size 0
```

`--duration` 是 CPU/内存采样窗口，不是整个命令的硬截止时间。Key 抽样仍在执行时，命令总耗时
可能更长，可使用 `Ctrl+C` 取消。

## 5. 常见问题判断

### 单个 master CPU 高

证据组合：

- 单个 master CPU 明显高于其他 master；
- 某命令调用或耗时占比高；
- 该 master 的 OPS、Key 数或内存偏高；
- 抽样槽位或大 Key 集中。

可能原因：

- 热点 Key 或热点 Hash Tag；
- HGETALL、SMEMBERS、LRANGE 等全量读取；
- 槽位或业务流量倾斜；
- 持久化或全量同步。

### 内存持续增长

证据组合：

- `used_memory` 持续增长；
- 无 TTL 样本比例高；
- master 之间内存偏差扩大；
- `evicted_keys` 开始增长。

可能原因：

- 数据生命周期管理缺失；
- 大 Key 或集合持续增长；
- 槽位/Hash Tag 集中；
- maxmemory 设置不合理。

### RSS 高但 used_memory 已下降

可能原因：

- 分配器碎片；
- 内存尚未归还操作系统；
- 大量对象删除后的驻留内存；
- 主机内存压力或 Swap。

不要只依据 Redis INFO 决定重启，应结合主机 RSS、Swap 和历史趋势。

### 客户端连接暴增

证据组合：

- 单一 IP 连接数或采样期连接增量超过阈值；
- 大量连接 age 较小；
- 连接名称为空或命令集中；
- Redis CPU 同时上升。

可能原因：

- 连接池失效；
- 客户端重试没有退避；
- 应用滚动发布或网络抖动；
- 健康检查频率过高。

## 6. 停止条件

出现下列任一情况应停止 `slots` 或 `diagnose`：

- Redis 命令延迟明显上升；
- replica 复制延迟持续扩大或链路断开；
- master CPU 接近饱和且仍在上升；
- 工具请求频繁超时；
- 业务错误率同步上升；
- 值班负责人要求停止采样。

停止后保留已生成报告，不要为了补齐样本立即重试。

## 7. 报告解读原则

- `high` 表示证据较强或风险较高，不等于已经确认根因；
- `confidence` 表示当前证据置信度；
- Key 和槽位分析是样本结论，不代表全量；
- SLOWLOG 只包含超过服务端阈值的命令；
- CLIENT LIST、SLOWLOG 和时间窗口可以形成线索，但不能默认建立“客户端 IP—Key”事实关系；
- RedisSleuth 的结论必须和 Prometheus、主机指标、应用日志或链路追踪交叉验证。

## 8. 事件记录模板

```text
事件时间：
集群/环境：
告警节点：
集群状态：
异常开始时间：
Redis CPU：
主机 CPU / iowait / Swap：
内存利用率与计算口径：
Top 命令：
新增慢日志：
持久化/复制状态：
客户端集中 IP：
Key/槽位抽样结果：
当前结论与置信度：
已采取措施：
后续负责人：
```
