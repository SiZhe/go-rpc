package main

import (
	"net"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// startServer 起一个模拟 C++ MPRPC 的 TCP 服务端,返回监听地址。
//   - alwaysFail=true:收到请求直接关连接(客户端读到空 → 失败),用于演示 per-node 熔断。
//   - alwaysFail=false:正常返回 StringValue("pong") 并短连接关闭。
func startServer(alwaysFail bool) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 4096)
			_, _ = conn.Read(buf)
			if alwaysFail {
				_ = conn.Close() // 制造失败
				continue
			}
			resp := wrapperspb.String("pong")
			b, _ := proto.Marshal(resp)
			_, _ = conn.Write(b)
			_ = conn.Close()
		}
	}()
	return ln.Addr().String()
}
