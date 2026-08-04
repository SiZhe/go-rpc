package transport

import (
	"net"
	"testing"

	"go-rpc/balancer"
	"go-rpc/discovery"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// startFakeMPRPCServer 起一个模拟 C++ MPRPC 服务端的 TCP server:
// 收到任意请求后,回写一个 StringValue("pong") 的 protobuf,然后关闭连接(短连接)。
func startFakeMPRPCServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0") // 0 = 系统分配空闲端口
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 4096)
			_, _ = conn.Read(buf)
			resp := wrapperspb.String("pong")
			b, _ := proto.Marshal(resp)
			_, _ = conn.Write(b)
			_ = conn.Close() // 短连接:发完即关,客户端读到 EOF
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func TestTCPTransportSendReceive(t *testing.T) {
	addr, stop := startFakeMPRPCServer(t)
	defer stop()

	reg := discovery.NewStaticRegistry(map[string][]string{
		"UserServiceRpc/Login": {addr},
	})
	tr := NewTCPTransport(reg, balancer.NewRoundRobin())

	frame, err := EncodeRequest("UserServiceRpc", "Login", []byte("req"), "trace-x")
	if err != nil {
		t.Fatal(err)
	}

	respBytes, err := tr.Send(rpccontext.New("UserServiceRpc", "Login"), frame)
	if err != nil {
		t.Fatal(err)
	}

	var resp wrapperspb.StringValue
	if err := proto.Unmarshal(respBytes, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Value != "pong" {
		t.Fatalf("resp = %q, want pong", resp.Value)
	}
}
