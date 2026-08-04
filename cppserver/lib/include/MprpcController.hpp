#pragma once
#include <google/protobuf/service.h>

class MprpcController : public google::protobuf::RpcController {
public:
    MprpcController();
    void Reset();
    bool Failed() const;
    std::string ErrorText() const;
    void StartCancel();

    void SetFailed(const std::string& reason);
    bool IsCanceled() const;
    void NotifyOnCancel(google::protobuf::Closure* callback);
private:
    bool failed_;
    std::string errText_;
};