#include "Logger.hpp"
#include <chrono>
#include <iostream>
#include <fstream>

Logger::Logger() {
    std::thread t1([&]() {
        for (;;) {
            // 获取当前日期，然后取日日志信息，写入日志文件
            time_t now = time(nullptr);
            tm* nowtm = localtime(&now);

            char filename[100];
            sprintf(filename,"%d-%d-%d-log.txt",nowtm->tm_year+1900,nowtm->tm_mon+1,nowtm->tm_mday);

            std::fstream file;
            file.open(filename,std::ios::app);
            if (!file) {
                std::cout << "logger file:" << filename << " open error!" << std::endl;
                exit(EXIT_FAILURE);
            }

            std::string msg = lckQue_.pop();

            char time[128];
            sprintf(time,"%d:%d:%d[%s]=>",nowtm->tm_hour,nowtm->tm_min,nowtm->tm_sec,(logLevel_ == INFO ? "info" : "error"));
            msg.insert(0,time);

            file << msg << std::endl;
            file.close();
        }
    });
    t1.detach();
}

Logger *Logger::getInstance() {
    static Logger instance;
    return &instance;
}

void Logger::setLogLevel(LogLevel level) {
    logLevel_ = level;
}

void Logger::Log(std::string msg) {
    lckQue_.push(msg);
}
