# cppserver — C++ MPRPC 服务端内核

这是 go-rpc 的 RPC 服务端内核(原 MPRPC 项目),基于 muduo + Protobuf + ZooKeeper。
go-rpc 的 Go 治理 SDK 作为客户端,通过 wire protocol 远程调用这里注册的服务。

## 依赖
- muduo(网络库)、protobuf、zookeeper C 客户端(zookeeper_mt)

## 构建
```bash
cd cppserver
bash autobuild.sh   # 或手动 cmake:
# mkdir build && cd build && cmake .. && make
```

## RpcHeader 协议(已为 go-rpc 扩展)
`src/RpcHeader.proto` 在原 3 个字段基础上新增:
- `trace_id`   跨进程链路追踪 ID
- `deadline_ms` 调用截止时间
- `metadata`   透传元数据

改动 proto 后需重新生成并编译:
```bash
cd src
protoc --cpp_out=. RpcHeader.proto
```

## 与 Go 客户端的 wire protocol
- 请求:`[4B 小端 header_size][RpcHeader][args]`
- 响应:直接回 response 的 protobuf,短连接发完即关。
- 服务注册:zk 路径 `/service_name/method_name` → `ip:port`。

Go 侧对应实现见 `../transport/`、`../discovery/`。
