#include "include/MprpcTask.hpp"

#include "RpcHeader.pb.h"

MprpcTask::MprpcTask(const std::string &recvBuf, const TcpConnectionPtr &conn, MprpcProvider *provider)
    : recv_buf(recvBuf), conn_(conn), provider_(provider) {}

Any MprpcTask::run() {
    // 从字节流中读取前四个字节(int 类型，记录服务名称和方法名称的长度)
    uint32_t header_size = 0;
    recv_buf.copy((char*)&header_size,4,0);

    // 根据header_size读取数据原始字符流
    std::string rpc_header_str = recv_buf.substr(4,header_size);

    // 反序列化
    mprpc::RpcHeader rpcHeader;
    std::string service_name;
    std::string method_name;
    uint32_t args_size;

    if (rpcHeader.ParseFromString(rpc_header_str)) {
        // 反序列化成功
        service_name = rpcHeader.service_name();
        method_name = rpcHeader.method_name();
        args_size = rpcHeader.args_size();
    }else {
        // 反序列化失败
        std::cout << "rpc_header_str:" << rpc_header_str << "parse error!" << std::endl;
        return "";
    }

    // 解析参数信息
    std::string args_str = recv_buf.substr(4+header_size,args_size);

    // 打印调试信息
    std::cout << "================================="<< std::endl;
    std::cout << "header_size:" << header_size << std::endl;
    std::cout << "rpc_header_str:" << rpc_header_str << std::endl;
    std::cout << "service_name:" << service_name << std::endl;
    std::cout << "method_name:" << method_name << std::endl;
    std::cout << "args_size:" << args_size << std::endl;
    std::cout << "args_str:" << args_str << std::endl;
    std::cout << "================================="<< std::endl;

    // 获取service 和 method对象
    auto sit = provider_->serviceMap_.find(service_name);
    if (sit == provider_->serviceMap_.end()) {
        std::cout << service_name << " not exist!" << std::endl;
        return "";
    }
    google::protobuf::Service* service = sit->second.service_;

    auto mit = sit->second.methodMap_.find(method_name);
    if (mit == sit->second.methodMap_.end()) {
        std::cout << method_name << " not exist!" << std::endl;
        return "";
    }
    const google::protobuf::MethodDescriptor* method = sit->second.methodMap_[method_name];

    // 生产rpc请求的request和response
    google::protobuf::Message* request = service->GetRequestPrototype(method).New();
    if (request->ParseFromString(args_str)) {

    }else {
        // 反序列化失败
        std::cout << "args_str:" << args_str << "parse error!" << std::endl;
        return "";
    }

    google::protobuf::Message* response = service->GetResponsePrototype(method).New();

    // 给CallMethod绑定一个Closure,通过网络将msg发出去
    google::protobuf::Closure* done = google::protobuf::NewCallback<MprpcProvider,
                                                                    const TcpConnectionPtr&,
                                                                    google::protobuf::Message*>
    (provider_,&MprpcProvider::sendRpcResponse,conn_,response);

    // 调用方法
    service->CallMethod(method,nullptr,request,response,done);
    return "success";
}

