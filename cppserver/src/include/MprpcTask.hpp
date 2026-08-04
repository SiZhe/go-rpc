#pragma once
#include <muduo/net/TcpServer.h>
#include <muduo/net/EventLoop.h>
#include "../include/ThreadPool.hpp"
#include "../include/MprpcProvider.hpp"

using namespace muduo;
using namespace muduo::net;

class MprpcTask : public Task {
public:
    MprpcTask(const std::string& reqData,
            const TcpConnectionPtr& conn,
            MprpcProvider* provider);

    Any run() override;

private:
    std::string recv_buf;          // RPC请求数据
    TcpConnectionPtr conn_;        // 客户端连接（智能指针，安全拷贝）
    MprpcProvider* provider_;      // RPC服务提供者（仅持有指针，需保证生命周期）
};
