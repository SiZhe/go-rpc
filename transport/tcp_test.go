package transport

import (
	"context"
	"net"
	"testing"

	"go-rpc/balancer"
	"go-rpc/discovery"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// startFakeMPRPCServer 起一个模拟 C++ MPRPC 服务端:收到请求后回 "pong" 的 protobuf,短连接关闭。
func startFakeMPRPCServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
			_ = conn.Close()
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
	defer tr.Close()

	frame, err := EncodeRequest("UserServiceRpc", "Login", []byte("req"), "trace-x", 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := rpccontext.New(context.Background(), "UserServiceRpc", "Login")
	respBytes, err := tr.Send(ctx, frame)
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
