#pragma once

#include <iostream>
#include <vector>
#include <queue>
#include <mutex>
#include <condition_variable>
#include <thread>
#include <memory>
#include <atomic>
#include <functional>
#include <map>

#define LOG(str) std::cout << __FILE__ << __LINE__ << __TIMESTAMP__ << " : " << str << std::endl
#define ERR(str) std::cerr << __FILE__ << __LINE__ << __TIMESTAMP__ << " : " << str << std::endl

//=================Any===================
class Any {
public:
    Any(const Any& src) = delete;
    Any& operator=(const Any& src) = delete;
    Any() = default;
    ~Any() = default;
    Any(Any&& src) = default;
    Any& operator=(Any&& src) = default;

    template<typename T>
    Any(T date):base_(std::make_unique<Drive<T>>(date)) {};

    template<typename T>
    T cast_() {
        Drive<T>* d = dynamic_cast<Drive<T>*>(base_.get());
        if (d == nullptr) {
            ERR("Type is unmatched when dynamic cast!");
        }
        return d->date_;
    }
private:
    class Base {
    public:
        Base(){};
        virtual ~Base() = default;
    };

    template<typename T>
    class Drive : public Base {
    public:
        Drive(T date) : Base(),date_(date){};
        T date_;
    };
public:
    std::unique_ptr<Base> base_;
};


//==============Semaphore=================
class Semaphore {
public:
    Semaphore(int res = 0);
    ~Semaphore() = default;

    void wait();

    void post();
private:
    int resCunt_;
    std::mutex mtx_;
    std::condition_variable cv_;
};

//=================Task===================
class Result;
class Task {
public:
    Task();
    virtual Any run() = 0 ;
    virtual ~Task() = default;

    void exec();
    void setResult(Result* result);
private:
    Result* result_;
};

//================Result==================
class Result {
public:
    Result(std::shared_ptr<Task> task,bool isValid = false);
    ~Result();

    Any get();
    void setValue(Any any);

    bool isValid() const;
private:

    Any any_; // 存储任务的返回值
    Semaphore sem; // 线程通信信号量
    std::shared_ptr<Task> task_;
    std::atomic_bool isValid_; // 返回值是否有效
};

//=================Thread===================
class Thread {
public:
    using ThreadFunc = std::function<void(int)>;
    Thread(ThreadFunc func);
    ~Thread();
    void start();

    int getThreadId() const;
private:
    static std::atomic_int incId_;
    int threadId_;
    ThreadFunc func_;
};

//===============ThreadPool================
enum class PoolMode {
    FIXED,
    CACHED
};

class ThreadPool {
public:
    ThreadPool();
    ~ThreadPool();

    // 设置参数
    // 设置线程池模式
    void setPoolMode(PoolMode mode);

    //设置线程池cached模式下线程最大数量
    void setMaxThreadSize(int maxThreadSize);

    //设置线程池cached模式下线程最大空闲时间
    void setMaxThreadIdleTime(int maxThreadIdleTime);

    //设置任务队列最大长度
    void setMaxTaskSize(int maxTaskSize);

    // 设置任务最大排队时间
    void setMaxSubmitTaskTime(int maxSubmitTaskTime);

    // 检查是否开启
    bool checkPoolState();

    // 开始线程池,默认为cpu的核心数-2
    void start(unsigned int initThreadSize = std::thread::hardware_concurrency()-2);
    // 提交任务
    Result submitTask(std::shared_ptr<Task> task);
private:
    void threadHandler(int threadId);
private:
    PoolMode poolMode_;
    bool isPoolRunning_;

    std::unordered_map<int,std::shared_ptr<Thread>> threadMap_;
    size_t initThreadSize_;
    size_t maxThreadSize_;
    std::atomic_int curThreadSize_;
    int maxThreadIdleTime_;
    std::atomic_int idleThreadSize_;

    std::queue<std::shared_ptr<Task>> taskQue_;
    size_t maxTaskSize_;
    std::atomic_int curTaskQueSize_;
    int maxSubmitTaskTime_;


    std::mutex mtx_;
    std::condition_variable cvTask_;
    std::condition_variable cvThread_;
    std::condition_variable cvQuit_;
};