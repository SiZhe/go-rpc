#include "MprpcConfig.hpp"
#include <fstream>

bool MprpcConfig::loadConfigFile(const char *config_file) {
    std::fstream file;
    file.open(config_file,std::ios::in);

    if (!file) {
        std::cout << "config file not open" << std::endl;
        return false;
    }

    std::string line;
    while (getline(file,line)) {
        if (line.empty() || line[0] == '#') {
            continue;
        }

        int idx = line.find("=");
        if (idx == -1) {
            continue;
        }

        std::string key = line.substr(0,idx);
        std::string value = line.substr(idx+1);

        if (key == "rpcserverip" || key == "rpcserverport" ||
            key == "zookeeperip" || key == "zookeeperport") {
            configMap_[key] = value;
        } else {
            std::cout << "Unknown config key:" << key << std::endl;
        }
    }
    file.close();
    return true;
}

std::string MprpcConfig::load(const std::string &key) {
    auto it = configMap_.find(key);
    if (it != configMap_.end()) {
        return it->second;
    }else {
        std::cout << "invalid key!" << std::endl;
        return "";
    }
}