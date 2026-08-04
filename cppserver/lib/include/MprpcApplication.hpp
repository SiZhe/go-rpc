#pragma once
#include "MprpcConfig.hpp"

// mprpc框架的基础类
class MprpcApplication {
public:
    MprpcApplication(const MprpcApplication&) = delete;
    MprpcApplication(MprpcApplication&&) = delete;

    static MprpcApplication* getMPRPC();

    static void Init(int argc,char** argv);

    static MprpcConfig& getConfig();
private:
    MprpcApplication();

    static MprpcConfig config_;
};

