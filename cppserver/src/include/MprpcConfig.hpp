#pragma once
#include <unordered_map>
#include <string>
#include <iostream>

class MprpcConfig {
public:
    // 加载配置文件
    bool loadConfigFile(const char* config_file);

    std::string load(const std::string& key);
private:
    std::unordered_map<std::string,std::string> configMap_;
};