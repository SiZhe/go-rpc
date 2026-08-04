#pragma once
#include <google/protobuf/service.h>
using namespace google::protobuf;

class MprpcChannel : public google::protobuf::RpcChannel{
public:
    void CallMethod(const MethodDescriptor* method,
                          RpcController* controller, const Message* request,
                          Message* response, Closure* done);
};
