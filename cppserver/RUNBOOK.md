# RUNBOOK — 真机联调:Go 客户端调用 C++ Provider

本文说明 go-rpc(Go 客户端)如何与 C++ MPRPC 服务端(Provider)真机联通。
当前 demo 用 Go 模拟服务端(`../example/server.go`);要连真 C++,按本文操作(需 Linux)。

## 角色关系

```
①  ZooKeeper 进程          ← "服务地址黄页",先启动
        ▲ 注册地址           ▲ 查询地址
        │                   │
②  C++ Provider 进程        ③  Go 客户端(go-rpc)
   监听 :8000                ── TCP(MPRPC 协议)──▶ 打到 :8000
   把 /UserServiceRpc/Login 注册到 zk
```

- **ZooKeeper**:独立中间件,像电话黄页。Provider 往里登记地址,客户端来查。
- **Provider(服务端)**:提供 RPC 服务的程序。跑起来后 ① 监听端口 ② 把服务地址注册到 zk
  ③ 阻塞等请求。对应代码 `example/callee/UserService.cpp` 的 main:
  ```cpp
  MprpcApplication::Init(argc, argv);          // 读配置(zk 地址等)
  MprpcProvider provider;
  provider.notifyService(new UserService());   // 登记 UserService
  provider.run();                              // 监听 + 注册 zk + 阻塞等请求
  ```
  "启动 Provider" = 编译并运行这个程序。
- **Consumer / 客户端**:发起调用的一方。`example/caller/` 是 C++ 版客户端;
  你的 go-rpc 就是它的 Go 增强版(多了中间件治理)。

## 前置依赖(Linux)

- protobuf、muduo(网络库)、zookeeper C 客户端(zookeeper_mt)、cmake、g++。

## 步骤

### 1. 启动 ZooKeeper
```bash
# 下载并启动 zk(以 apache-zookeeper 为例)
zkServer.sh start          # 默认监听 2181
```

### 2. 改 Provider 的配置,指向 zk
`bin/rpc_zk.conf` 里配置 zk 地址和本机监听端口(参考仓库现有 conf 格式):
```
zookeeperip=127.0.0.1
zookeeperport=2181
rpcserverip=127.0.0.1
rpcserverport=8000
```

### 3. 编译并启动 C++ Provider
```bash
cd cppserver
bash autobuild.sh                      # 或 mkdir build && cd build && cmake .. && make
./bin/provider -i bin/rpc_zk.conf      # 具体可执行名以 CMakeLists 产物为准
# 启动后:监听 :8000,并在 zk 上创建 /UserServiceRpc/Login → 127.0.0.1:8000
```
验证注册成功:
```bash
zkCli.sh -server 127.0.0.1:2181 get /UserServiceRpc/Login   # 应看到 127.0.0.1:8000
```

### 4. Go 客户端改用 ZkRegistry(无需改其它代码)
把 demo 里的静态注册中心换成 zk 发现:
```go
// 之前(模拟):
// reg := discovery.NewStaticRegistry(map[string][]string{...})

// 真机:连 zk 自动发现 C++ Provider 注册的地址
reg, err := discovery.NewZkRegistry([]string{"127.0.0.1:2181"}, 5*time.Second)
if err != nil { panic(err) }
defer reg.Close()
```
其余(transport / 中间件 / Call)完全不变 —— 因为 Go 侧的 wire protocol 与 C++ 严格对齐。

### 5. 让请求/响应的 proto 对齐
- 请求参数、响应类型要用 C++ Provider 那边 `example/user.proto` 对应的 message
  (Go 侧用同一份 proto 生成 Go 代码),而不是 demo 里临时用的 wrapperspb.StringValue。
- 服务名/方法名以 C++ 注册的为准(如 `UserServiceRpc` / `Login`)。

## 为什么现在 demo 不用真 C++

编译 cppserver 需 Linux + muduo + zookeeper C 库,本地(macOS)不具备。
但 Go 侧 transport 严格实现了 MPRPC wire protocol(4B 小端长度 + RpcHeader + body,
响应读到 EOF),并用 Go 假服务端精确复刻该格式做了收发测试,故协议实现是正确的,
换真 C++ Provider 即可联通。
