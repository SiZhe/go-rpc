#include "MprpcProvider.hpp"
#include "MprpcApplication.hpp"
#include "include/RpcHeader.pb.h"
#include "ZookeeperUtil.hpp"
#include "MprpcTask.hpp"

using namespace std::placeholders;

void MprpcProvider::notifyService(google::protobuf::Service *service) {
    ServiceInfo sInfo;
    sInfo.service_ = service;
    // 获取服务对象的描述信息
    const google::protobuf::ServiceDescriptor* serviceDesc = service->GetDescriptor();
    // 获取方法名称
    std::string sName = serviceDesc->name();
    std::cout << "service name:" << sName << std::endl;
    // 获取方法数量
    int count = serviceDesc->method_count();

    for (int i = 0 ; i < count ; i++) {
        const google::protobuf::MethodDescriptor* pMethodDesc = serviceDesc->method(i);

        std::string pName = pMethodDesc->name();
        std::cout << "  method name:" << pName << std::endl;
        sInfo.methodMap_[pName] = pMethodDesc;
    }

    serviceMap_[sName] = sInfo;
}

void MprpcProvider::run() {
    std::string ip = MprpcApplication::getConfig().load("rpcserverip");
    int port = std::stoi(MprpcApplication::getConfig().load("rpcserverport"));
    InetAddress address(ip,port);

    // 创建tcpserver对象
    TcpServer tcp(&eventLoop_,address,"RpcProvider");

    // 绑定连接回调
    tcp.setConnectionCallback(std::bind(&MprpcProvider::onConnect,this,_1));

    // 绑定信息回调
    tcp.setMessageCallback(std::bind(&MprpcProvider::onMessage,this,_1,_2,_3));

    // 设置线程数量 1个i/o线程，3个worker线程
    tcp.setThreadNum(4);

    std::cout << "RpcProvider start service at ip:" << ip <<" port:" << port << std::endl;

    ZkClient zkcli;
    zkcli.Start();

    for (auto& sp : serviceMap_) {
        std::string service_path = "/" + sp.first;
        zkcli.Create(service_path.c_str(),nullptr,0);
        for (auto& mp : sp.second.methodMap_) {
            std::string method_path = service_path + "/" + mp.first;
            char method_path_data[128];
            sprintf(method_path_data,"%s:%d",ip.c_str(),port);
            // std::cout << strlen(method_path_data) << std::endl;
            // 临时性节点
            zkcli.Create(method_path.c_str(),method_path_data,static_cast<int>(strlen(method_path_data)),ZOO_EPHEMERAL);
        }
    }

    // 启动网络服务
    tcp.start();
    eventLoop_.loop();

    threadPool_->setPoolMode(PoolMode::CACHED);
    threadPool_->start();
}

void MprpcProvider::onConnect(const TcpConnectionPtr & conn) {
    if (!conn->connected()) {
        conn->shutdown();
    }
}

void MprpcProvider::onMessage(const TcpConnectionPtr & conn, Buffer * buf , Timestamp time) {
    std::string recv_buf = buf->retrieveAllAsString();

    std::shared_ptr<MprpcTask> task = std::make_shared<MprpcTask>(recv_buf,conn,this);
    auto result = threadPool_->submitTask(task);

    if (!result.isValid()) {
        LOG("RPC task submit failed! Task queue is full!");
        // 给客户端返回错误响应
        conn->send("Server busy, try again later!");
        conn->shutdown();
    }
}

void MprpcProvider::sendRpcResponse(const TcpConnectionPtr & conn, google::protobuf::Message *response) {
    std::string response_str;
    if (response->SerializeToString(&response_str)) {
        conn->send(response_str);
    }else {
        std::cout << "sendRpcResponse serialize error!!" << std::endl;
    }

    // 短连接，send后就断开
    conn->shutdown();
}
