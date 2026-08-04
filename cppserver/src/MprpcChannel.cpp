#include "MprpcChannel.hpp"
#include <google/protobuf/descriptor.h>
#include <google/protobuf/message.h>
#include <netinet/in.h>

#include "RpcHeader.pb.h"
#include "MprpcApplication.hpp"
#include "MprpcController.hpp"
#include "ZookeeperUtil.hpp"

#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>

#include "user.pb.h"

/*
header_size + service_name method_name args_size + args
*/
// 所有通过stub代理对象调用的rpc方法，都走到这里了，统一做rpc方法调用的数据数据序列化和网络发送

void MprpcChannel::CallMethod(const MethodDescriptor *method,
                              RpcController *controller,
                              const Message *request,
                              Message *response,
                              Closure *done) {
    const ServiceDescriptor* sd =  method->service();
    std::string service_name = sd->name();
    std::string method_name = method->name();

    // 获取参数序列化的长度
    size_t args_size = 0;
    std::string args_str;
    if (request->SerializeToString(&args_str)) {
        args_size = args_str.size();
    }else {
        controller->SetFailed("serialize request error!");
        return;
    }

    // 定义请求头
    mprpc::RpcHeader header;
    header.set_service_name(service_name);
    header.set_method_name(method_name);
    header.set_args_size(args_size);

    uint32_t header_size;
    std::string rpc_header_str;
    if (header.SerializeToString(&rpc_header_str)) {
        header_size = rpc_header_str.size();
    }else {
        controller->SetFailed("serialize rpcHeader error!");
        return;
    }

    std::string rpc_send_str;
    // 先写4个字节的长度
    rpc_send_str.insert(0,std::string((char*)&header_size,4));
    // 再写头部信息
    rpc_send_str.append(rpc_header_str);
    // 最后是真正的参数
    rpc_send_str.append(args_str);

    // 打印调试信息
    std::cout << "================================="<< std::endl;
    std::cout << "header_size:" << header_size << std::endl;
    std::cout << "rpc_header_str:" << rpc_header_str << std::endl;
    std::cout << "service_name:" << service_name << std::endl;
    std::cout << "method_name:" << method_name << std::endl;
    std::cout << "args_size:" << args_size << std::endl;
    std::cout << "args_str:" << args_str << std::endl;
    std::cout << "================================="<< std::endl;

    // 使用tcp编程，完成rpc的远程调用
    int clientfd = socket(AF_INET,SOCK_STREAM,0);

    if (clientfd == -1) {
        controller->SetFailed("create clientfd failed!");
        return;
    }

    ZkClient zkcli;
    zkcli.Start();

    std::string method_path = "/" + service_name + "/" + method_name;
    std::string method_host = zkcli.GetData(method_path.c_str());

    if (method_host.empty()) {
        controller->SetFailed(method_host+ "is not exist!");
        return;
    }

    int idx = method_host.find(":");
    if (idx == -1) {
        controller->SetFailed(method_host+ "is invalid!");
        return;
    }
    std::string ip = method_host.substr(0,idx);
    uint16_t port = std::stoi(method_host.substr(idx+1));

    sockaddr_in server_addr;
    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(port);
    server_addr.sin_addr.s_addr = inet_addr(ip.c_str());

    if (connect(clientfd,(sockaddr*)&server_addr,sizeof(server_addr)) == -1) {
        controller->SetFailed("connect server failed!");
        close(clientfd);
        return;
    }

    if (send(clientfd,rpc_send_str.c_str(),rpc_send_str.size(),0) == -1) {
        controller->SetFailed("send message failed!");
        close(clientfd);
        return;
    }

    char recv_buf[1024];
    int recv_size;
    if ((recv_size = recv(clientfd,&recv_buf,1024,0)) == -1) {
        controller->SetFailed("recv message failed!");
        close(clientfd);
        return;
    }

    //std::string response_str(recv_buf,0,recv_size);
    if (!response->ParseFromArray(recv_buf,recv_size)) {
        controller->SetFailed("response parse failed!");
    }

    close(clientfd);
}
