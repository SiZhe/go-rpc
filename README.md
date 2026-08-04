# go-rpc

一个用 **Go 实现的客户端治理 SDK**,为 C++ RPC 框架 [MPRPC](../MPRPC) 提供
微服务治理与可观测能力。设计理念参考 B 站 go-common / kratos。

> C++ MPRPC 负责高性能 RPC 服务端内核(muduo + Protobuf + ZooKeeper);
> go-rpc 作为客户端,通过 MPRPC 的 wire protocol(Protobuf over TCP)远程调用它,
> 并在调用链路上叠加超时、重试、熔断、限流、负载均衡、链路追踪、指标、日志等能力。

## 快速开始

```bash
cd go-rpc
go test ./...      # 跑全部单元测试
go run ./example   # 跑完整 demo:组装所有中间件,发起真实 TCP 调用
```

## 核心设计:中间件洋葱模型

整个框架的灵魂是 `middleware` 包里 10 行的 `Chain`。一次 RPC 调用像剥洋葱一样,
从外层中间件一层层进入,到最内层真正发网络请求,再一层层返回:

```
请求 →│Trace│AccessLog│Metric│RateLimit│Breaker│Retry│Timeout│→ TCP → C++ 服务端
      └──────────── 每一层都能在调用前后做事 ────────────┘
```

- 框架核心只负责"串链"(`Chain`),不含任何业务/治理逻辑。
- 每个治理能力都是一个独立中间件(`middleware.Middleware` 类型),想要哪个挂哪个。
- 这就是 kratos / go-common 的扩展方式:**核心稳定,能力横向叠加**。

## 目录结构

| 包 | 职责 | 关键面试点 |
|---|---|---|
| `middleware/` | 中间件链引擎(Handler/Middleware/Chain) | 洋葱模型、高阶函数 |
| `rpccontext/` | 调用上下文(TraceID/Deadline/Metadata 载体) | context 贯穿链路 |
| `transport/` | wire 编解码 + 真实 TCP 传输 | 协议对齐、TCP 粘包、短连接分帧 |
| `discovery/` | 服务发现(Static / ZooKeeper) | 面向接口、服务注册发现 |
| `balancer/` | 负载均衡(轮询/随机/平滑加权) | 平滑加权轮询算法 |
| `breaker/` | 熔断器(滑窗三态机) | Closed/Open/HalfOpen、滑动窗口 |
| `ratelimit/` | 限流(令牌桶) | 令牌桶 vs 漏桶、惰性补令牌 |
| `trace/` | TraceID/SpanID 生成 | 分布式追踪、跨进程透传 |
| `metric/` | 指标聚合(QPS/耗时/错误率) | 并发安全聚合 |
| `log/` | 结构化异步日志 | 异步日志、TraceID 关联 |
| `middlewares/` | 把上述能力包成可挂载的中间件 | 引擎 vs 挂件分离 |
| `example/` | 组装全部能力的完整 demo | — |

## 一次调用的完整链路(数据流)

```
cli.Call(ctx, "UserServiceRpc", "Login", req, resp)
  → Trace:      生成 TraceID 写入 ctx
  → AccessLog:  记录开始时间
  → Metric:     记录开始时间
  → RateLimit:  令牌桶取令牌,没令牌直接拒
  → Breaker:    熔断打开则快速失败
  → Retry:      失败则指数退避重试
  → Timeout:    goroutine + select 加超时,把 deadline 写入 ctx
  → transport:  proto.Marshal(req) → EncodeRequest 组 wire 帧
                → discovery 查地址 → balancer 选址 → TCP 发送
                → 读到 EOF 收完整 response → proto.Unmarshal 到 resp
  ← 逐层返回,AccessLog/Metric 记录耗时与结果
```

## 中间件顺序为什么这么排

洋葱从外到内的顺序决定"谁先执行"。一个通用原则:

- **限流、熔断靠外**:尽早拦截无效请求,不浪费内层和网络开销(fail-fast)。
- **重试在熔断内侧**:重试产生的失败也应计入熔断统计;否则重试会绕过熔断保护。
- **超时最内**:紧贴真正的网络调用,截止时间通过 ctx 传到 transport 的 socket deadline。
- **Trace 最外**:先生成 TraceID,后续所有中间件(日志、指标)和下游都能用同一个 ID。

## 可观测性三支柱如何联动(项目亮点)

Trace / Metric / Log 三者用**同一个 TraceID** 关联:

- `trace` 中间件在入口生成 TraceID,写进 `ctx.TraceID`。
- `transport` 把它编码进 `RpcHeader.trace_id`,跨进程/跨语言透传给 C++ 服务端。
- `log` 每条日志都带 TraceID;`metric` 按方法维度聚合。
- 线上排障时,一个 TraceID 就能把"客户端日志 + 链路 + 指标 + 服务端日志"全部串起来。

## 与 C++ MPRPC 的协议对齐(关键细节)

- 请求帧:`[4 字节小端 header_size][RpcHeader(protobuf)][args(protobuf)]`
  —— 与 C++ `MprpcChannel::CallMethod` 的组包逐字节一致(小端来自 C++ 直接拷贝 uint32 内存)。
- 响应:MPRPC 服务端发完 response 的 protobuf 就关闭连接(短连接)。
  Go 侧读到 EOF,读到的全部字节即完整 response —— 用"连接关闭"天然分帧。
- 服务发现:ZooKeeper 路径 `/service_name/method_name` → `ip:port`,与 C++ 注册结构一致。
- 为支持跨进程链路追踪,给 `RpcHeader` 新增了 `trace_id / deadline_ms / metadata` 字段
  (C++ 侧同步改 proto 并重新编译)。

## 学习路径建议

按依赖顺序读,由简到难:
1. `rpccontext` → `middleware`(理解洋葱模型,全项目的地基)
2. `transport/codec.go`(协议编解码)→ `transport/tcp.go`(真实网络)
3. `discovery` → `balancer`(服务发现与选址)
4. `middlewares/timeout.go` → `retry.go`(简单中间件)
5. `breaker/`(重点!滑窗三态机)→ `ratelimit/`(令牌桶)
6. `trace/` → `metric/` → `log/`(可观测性)
7. `example/main.go`(看全部如何组装联动)
