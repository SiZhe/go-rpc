#pragma once

#include "LockQueue.hpp"

enum LogLevel {
    INFO,
    ERROR,
};

class Logger {
public:
    static Logger* getInstance();
    Logger(const Logger&)=delete;
    Logger(Logger&&)= delete;
    Logger& operator=(const Logger&)=delete;

    void setLogLevel(LogLevel level);
    void Log(std::string msg);
private:
    Logger();
    int logLevel_;
    LockQueue<std::string> lckQue_;
};

// 定义宏 LOG_INFO("xxx %d %s" ,20,"xxx");
#define LOG_INFO(logmsgformat, ...) \
        do \
        { \
        Logger* logger = Logger::getInstance(); \
        logger->setLogLevel(LogLevel::INFO); \
        char c[1024]; \
        snprintf(c, sizeof(c), logmsgformat, ##__VA_ARGS__); \
        logger->Log(c); \
        } while (0)

#define LOG_ERROR(logmsgformat, ...) \
        do \
        { \
        Logger* logger = Logger::getInstance(); \
        logger->setLogLevel(LogLevel::ERROR); \
        char c[1024]; \
        snprintf(c, sizeof(c), logmsgformat, ##__VA_ARGS__); \
        logger->Log(c); \
        } while (0)