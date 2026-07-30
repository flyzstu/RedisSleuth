# RedisSleuth MVP 设计

RedisSleuth 从一个入口执行 `CLUSTER NODES` 和 `CLUSTER INFO`，随后为可访问节点创建独立的
只读客户端。每次请求都受 `command_timeout` 限制，连接也有单独超时。采集器只暴露 INFO、
CLUSTER、CLIENT、SLOWLOG、SCAN、TYPE、PTTL、MEMORY USAGE 和只读长度命令。

CPU 通过两个快照间的 `used_cpu_sys + used_cpu_user` 差值除以实际时间计算，因此结果是 Redis
进程消耗的单核 CPU 百分比，可能超过 100%（后台线程或版本行为也会影响累计口径）。内存优先
以 `maxmemory` 为分母；未设置时以 `total_system_memory` 为分母并显式标记。

Key 抽样按 master 分组，优先在其在线 replica 上执行有速率限制的 SCAN。槽位使用 Redis
CRC16/XMODEM 和 Hash Tag 规则在本地计算。规则引擎只基于同一窗口内的证据生成 finding，
不会推断某客户端访问了某个具体 Key。

Redis 5.0 兼容策略：

- 使用 RESP2 和 Redis 5 已存在的只读命令；
- `username: default` 转换为 Redis 5 可接受的 `AUTH <password>`；
- Redis 5 缺少的 allocator/client-memory INFO 字段按未知（零值）处理；
- 不使用 Redis 6 ACL、Redis 7 command metadata 或任何写命令。
