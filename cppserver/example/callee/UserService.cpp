#include <iostream>
#include <string>
#include "user.pb.h"
#include "MprpcApplication.hpp"
#include "MprpcProvider.hpp"
#include "Logger.hpp"

class UserService : public fixbug::UserServiceRpc{
public:
    bool Login(std::string name, std::string pwd){
        std::cout << "doing local service: Login" << std::endl;
        std::cout << "name:" << name << " pwd:" << pwd << std::endl;
        return true;
    }

    bool Register(uint32_t id, std::string name, std::string pwd){
        std::cout << "doing local service: Register" << std::endl;
        std::cout << "id:" << id << "name:" << name << " pwd:" << pwd << std::endl;
        return true;
    }

    /*
    重写基类UserServiceRpc的虚函数 下面这些方法都是框架直接调用的
    1. caller   ===>   Login(LoginRequest)  => muduo =>   callee
    2. callee   ===>    Login(LoginRequest)  => 交到下面重写的这个Login方法上了
    */
    void Login(::google::protobuf::RpcController* controller,
                       const ::fixbug::LoginRequest* request,
                       ::fixbug::LoginResponse* response,
                       ::google::protobuf::Closure* done) {
        // 框架给业务上报了请求参数LoginRequest，应用获取相应数据做本地业务
        std::string name = request->name();
        std::string pwd = request->pwd();

        // 做本地业务
        bool login_result = Login(name, pwd);

        response->set_sucess(login_result);

        fixbug::ResultCode *result = response->mutable_result();
        result->set_errcode(0);
        result->set_errmsg("");

        // 执行回调操作   执行响应对象数据的序列化和网络发送（都是由框架来完成的）
        done->Run();
    }

    void Register(::google::protobuf::RpcController* controller,
                         const ::fixbug::RegisterRequest* request,
                         ::fixbug::RegisterResponse* response,
                         ::google::protobuf::Closure* done){
        uint32_t id = request->id();
        std::string name = request->name();
        std::string pwd = request->pwd();

        bool register_result = Register(id,name,pwd);

        response->set_sucess(register_result);

        fixbug::ResultCode* result_code = response->mutable_result();
        result_code->set_errcode(0);
        result_code->set_errmsg("");

        done->Run();
    }
};

int main (int argc ,char** argv) {
    LOG_INFO("first log msg!");

    // 调用框架的初始化操作
    MprpcApplication::Init(argc,argv);

    // provider是一个rpc网络服务对象。把UserService对象发布到rpc节点上
    MprpcProvider provider{};
    provider.notifyService(new UserService());

    // 启动一个rpc服务发布节点   Run以后，进程进入阻塞状态，等待远程的rpc调用请求
    provider.run();

    return 0;
}