#include <iostream>
#include "MprpcApplication.hpp"
#include "user.pb.h"
#include "MprpcChannel.hpp"
#include "MprpcController.hpp"

int main(int argc, char **argv) {
    // 整个程序启动以后，想使用mprpc框架来享受rpc服务调用，一定需要先调用框架的初始化函数（只初始化一次）
    MprpcApplication::Init(argc, argv);

    // 演示调用远程发布的rpc方法Login
    fixbug::UserServiceRpc_Stub stub(new MprpcChannel());

    // rpc方法的请求参数
    fixbug::LoginRequest request;
    request.set_name("lisizhe");
    request.set_pwd("002522");

    // rpc方法的响应
    fixbug::LoginResponse response;

    MprpcController controller;

    // 发起rpc方法的调用  同步的rpc调用过程  MprpcChannel::callmethod
    stub.Login(&controller, &request, &response, nullptr); // RpcChannel->RpcChannel::callMethod 集中来做所有rpc方法调用的参数序列化和网络发送

    // 一次rpc调用完成，读调用的结果
    if (controller.Failed()) {
        std::cout << controller.ErrorText() << std::endl;
    }else {
        if (0 == response.result().errcode()) {
            std::cout << "rpc login response success:" << response.sucess() << std::endl;
        }else {
            std::cout << "rpc login response error : " << response.result().errmsg() << std::endl;
        }
    }

    fixbug::RegisterRequest regRequest;
    regRequest.set_id(1);
    regRequest.set_name("lisizhe");
    regRequest.set_pwd("002522");

    fixbug::RegisterResponse regResponse;


    stub.Register(nullptr,&regRequest,&regResponse,nullptr);

    // 一次rpc调用完成，读调用的结果
    if (0 == regResponse.result().errcode()) {
        std::cout << "rpc register response success:" << regResponse.sucess() << std::endl;
    }else {
        std::cout << "rpc register response error : " << regResponse.result().errmsg() << std::endl;
    }

    return 0;
}