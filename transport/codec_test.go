package transport

import (
	"encoding/binary"
	"testing"

	pb "go-rpc/proto"
	"google.golang.org/protobuf/proto"
)

func TestEncodeRequestLayout(t *testing.T) {
	args := []byte("hello-args")
	frame, err := EncodeRequest("UserServiceRpc", "Login", args, "trace-123", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	headerSize := binary.LittleEndian.Uint32(frame[:4])
	var h pb.RpcHeader
	if err := proto.Unmarshal(frame[4:4+headerSize], &h); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if string(h.ServiceName) != "UserServiceRpc" || string(h.MethodName) != "Login" {
		t.Fatalf("header svc/method = %s/%s", h.ServiceName, h.MethodName)
	}
	if h.TraceId != "trace-123" {
		t.Fatalf("trace = %s", h.TraceId)
	}
	if int(h.ArgsSize) != len(args) {
		t.Fatalf("args_size = %d, want %d", h.ArgsSize, len(args))
	}
	if string(frame[4+headerSize:]) != "hello-args" {
		t.Fatalf("args tail = %q", frame[4+headerSize:])
	}
}
