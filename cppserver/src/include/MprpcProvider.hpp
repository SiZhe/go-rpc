#pragma once
#include <google/protobuf/service.h>
#include <google/protobuf/descriptor.h>
#include <memory>
#include <muduo/net/TcpServer.h>
#include <muduo/net/EventLoop.h>
#include <muduo/net/InetAddress.h>
#include <unordered_map>
#include "ThreadPool.hpp"

using namespace muduo;
using namespace muduo::net;

class MprpcTask;

// 框架提供的专门服务发布rpc服务的网络对象类
class MprpcProvider {
public:
    // 给外部使用的，可以发布rpc方法的函数接口，用基类接收
    void notifyService(google::protobuf::Service* service);

    // 启动rpc服务节点，开始提供rpc远程调用服务
    void run();

private:
    void onConnect(const TcpConnectionPtr&);
    void onMessage(const TcpConnectionPtr&,Buffer*,Timestamp);

    // Closure回调操作，用于序列化rpc的响应和网络发送
    void sendRpcResponse(const TcpConnectionPtr&, google::protobuf::Message* response);
private:
    // 线程池对象
    std::unique_ptr<ThreadPool> threadPool_;

    std::unique_ptr<TcpServer> tcpServerPtr_;
    EventLoop eventLoop_;

    struct ServiceInfo {
        google::protobuf::Service* service_;
        std::unordered_map<std::string,const google::protobuf::MethodDescriptor*> methodMap_;
    };

    // 保存注册成功的服务的信息
    std::unordered_map<std::string, ServiceInfo> serviceMap_;

    friend class MprpcTask;
};
