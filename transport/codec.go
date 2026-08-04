package transport

import (
	"encoding/binary"

	pb "go-rpc/proto"
	"google.golang.org/protobuf/proto"
)

// EncodeRequest 按 MPRPC wire 格式组包:[4B 小端 header_size][RpcHeader][args]。
// args 为调用方已序列化好的请求体。traceID / deadlineMs / metadata 允许为零值。
// deadlineMs 为 0 表示无超时;metadata 为 nil 表示无透传元数据。
func EncodeRequest(service, method string, args []byte, traceID string, deadlineMs int64, metadata map[string]string) ([]byte, error) {
	header := &pb.RpcHeader{
		ServiceName: []byte(service),
		MethodName:  []byte(method),
		ArgsSize:    uint32(len(args)),
		TraceId:     traceID,
		DeadlineMs:  deadlineMs,
		Metadata:    metadata,
	}
	headerBytes, err := proto.Marshal(header)
	if err != nil {
		return nil, err
	}
	frame := make([]byte, 4+len(headerBytes)+len(args))
	binary.LittleEndian.PutUint32(frame[:4], uint32(len(headerBytes)))
	copy(frame[4:], headerBytes)
	copy(frame[4+len(headerBytes):], args)
	return frame, nil
}

// DecodeResponse 解析服务端响应。MPRPC 服务端短连接下直接回写 response protobuf
// (无长度前缀),因此调用方读到 EOF 的整块字节即 respBytes,这里透传给上层反序列化。
func DecodeResponse(respBytes []byte) []byte {
	return respBytes
}
