#include "MprpcApplication.hpp"
#include <iostream>
#include <unistd.h>

MprpcConfig MprpcApplication::config_;

MprpcApplication::MprpcApplication() {

}

MprpcApplication *MprpcApplication::getMPRPC() {
    static MprpcApplication instance;
    return &instance;
}

MprpcConfig &MprpcApplication::getConfig() {
    return config_;
}

void showArgsHelp() {
    std::cout << "format: command -i <configfile>" << std::endl;
}

// 静态成员函数只能范围静态成员变量
void MprpcApplication::Init(int argc, char **argv) {
    if (argc < 2) {
        showArgsHelp();
        exit(1);
    }
    int c = 0;
    std::string config_file;
    // i 表示要i这个指令， ：表示i后面要跟参数
    while ((c = getopt(argc,argv,"i:")) != -1) {
        switch (c) {
            case 'i': {
                config_file = optarg;
            }
                break;
            case '?': {
                showArgsHelp();
            }
                exit(1);
            case ':': {
                showArgsHelp();
            }
                exit(1);
            default:
                break;
        }
    }

    // 开始加载配置文件
    if(!config_.loadConfigFile(config_file.c_str())) {
        exit(1);
    }
}
