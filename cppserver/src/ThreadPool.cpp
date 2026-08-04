#include <algorithm>
#include "../include/ThreadPool.hpp"

//==============Semaphore=================
Semaphore::Semaphore(int res) : resCunt_(res){

}

void Semaphore::post() {
    std::unique_lock<std::mutex> lck(mtx_);
    resCunt_++;
    cv_.notify_all();
}

void Semaphore::wait() {
    std::unique_lock<std::mutex> lck(mtx_);
    cv_.wait(lck,[&]() {
       return resCunt_ > 0;
    });
    resCunt_--;
}

//=================Task===================
Task::Task()
    :result_(nullptr){

}

void Task::setResult(Result *result) {
    result_ = result;
}

void Task::exec() {
    if (result_ != nullptr) {
        Any any = run();
        result_->setValue(std::move(any));
    }
}

//================Result==================
Result::Result(std::shared_ptr<Task> task, bool isValid)
    :task_(task),
    isValid_(isValid){
    task->setResult(this);
}

Result::~Result() {

}


Any Result::get() {
    if (!isValid_) {
        return "";
    }

    sem.wait(); // task 如果没有执行完，会阻塞，等待资源不为0
    return std::move(any_);
}

void Result::setValue(Any any) {
    any_ = std::move(any);
    sem.post(); // 增加信号量资源；
}

bool Result::isValid() const {
    return isValid_;
}



//=================Thread===================
std::atomic_int Thread::incId_ = 0;

Thread::Thread(ThreadFunc func)
    :threadId_(++incId_),
    func_(func){

}

Thread::~Thread() {

}

int Thread::getThreadId() const {
    return threadId_;
}

void Thread::start() {
    std::thread t(func_,threadId_);
    t.detach();
}

//===============ThreadPool================
const int THREAD_MAX_IDLE_TIME = 4;
const int THREAD_MAX_SIZE = 10;
const int TASK_MAX_SUBMIT_TIME = 1;
const int TASK_MAX_SIZE = 10;

ThreadPool::ThreadPool()
    :poolMode_(PoolMode::FIXED),
    isPoolRunning_(false),

    initThreadSize_(0),
    maxThreadSize_(THREAD_MAX_SIZE),
    curThreadSize_(0),
    maxThreadIdleTime_(THREAD_MAX_IDLE_TIME),
    idleThreadSize_(0),

    maxTaskSize_(TASK_MAX_SIZE),
    curTaskQueSize_(0),
    maxSubmitTaskTime_(TASK_MAX_SUBMIT_TIME){

}

ThreadPool::~ThreadPool() {
    isPoolRunning_ = false;
    std::unique_lock<std::mutex> lck(mtx_);
    cvThread_.notify_all();
    cvQuit_.wait(lck,[&]() {
        return curThreadSize_ == 0;
    });

}

bool ThreadPool::checkPoolState() {
    return isPoolRunning_;
}

void ThreadPool::setPoolMode(PoolMode mode) {
    if (checkPoolState()) {
        LOG("Pool is running, can't set ! ! !");
        return;
    }
    poolMode_ = mode;
}

void ThreadPool::setMaxThreadSize(int maxThreadSize) {
    if (checkPoolState()) {
        LOG("Pool is running, can't set ! ! !");
        return;
    }
    if (poolMode_ == PoolMode::FIXED) {
        LOG("Pool mode is FIXED, thread size is fixed ! ! !");
        return;
    }
    maxThreadSize_ = maxThreadSize;
}

void ThreadPool::setMaxThreadIdleTime(int maxThreadIdleTime) {
    if (checkPoolState()) {
        LOG("Pool is running, can't set ! ! !");
        return;
    }
    if (poolMode_ == PoolMode::FIXED) {
        LOG("Pool mode is FIXED, thread will not delete ! ! !");
        return;
    }
    maxThreadIdleTime_ = maxThreadIdleTime;
}

void ThreadPool::setMaxTaskSize(int maxTaskSize) {
    if (checkPoolState()) {
        LOG("Pool is running, can't set ! ! !");
        return;
    }
    maxTaskSize_ = maxTaskSize;
}

void ThreadPool::setMaxSubmitTaskTime(int maxSubmitTaskTime) {
    if (checkPoolState()) {
        LOG("Pool is running, can't set ! ! !");
        return;
    }
    maxSubmitTaskTime_ = maxSubmitTaskTime;
}

void ThreadPool::start(unsigned int initThreadSize) {
    // 设置线程池的状态
    isPoolRunning_ = true;
    initThreadSize_ = initThreadSize;

    for (int i = 0 ; i < initThreadSize_ ; ++i) {
        auto threadPtr = std::make_shared<Thread>(std::bind(&ThreadPool::threadHandler,this,std::placeholders::_1));
        int threadId = threadPtr->getThreadId();
        threadMap_[threadId] = threadPtr;
        ++curThreadSize_;
        idleThreadSize_++;
    }

    for (auto& thread : threadMap_) {
        thread.second->start();
    }
}

void ThreadPool::threadHandler(int threadId) {
    auto lastIdleTime = std::chrono::system_clock::now();
    for (;;) {

        std::shared_ptr<Task> task;
        // 局部运行实现锁的自动释放
        {
            std::unique_lock<std::mutex> lck(mtx_);

            while (curTaskQueSize_ == 0) {

                if (!checkPoolState()) {
                    std::cout << "线程" << threadId << "被关闭。。。。" << std::endl;
                    threadMap_.erase(threadId);
                    // 数量减少
                    curThreadSize_--;
                    idleThreadSize_--;
                    cvQuit_.notify_all();
                    return;
                }

                if (poolMode_ == PoolMode::FIXED) {
                    cvThread_.wait(lck);
                }else if (poolMode_ == PoolMode::CACHED) {
                    auto flag = cvThread_.wait_for(lck,std::chrono::seconds(1));
                    if (flag == std::cv_status::timeout) {
                        auto now = std::chrono::system_clock::now();
                        auto dur = std::chrono::duration_cast<std::chrono::seconds>(now - lastIdleTime);

                        if (dur.count() > maxThreadIdleTime_ && curThreadSize_ > initThreadSize_) {
                            std::cout << "线程" << threadId << "已被回收。。。。" << std::endl;
                            threadMap_.erase(threadId);
                            idleThreadSize_--;
                            curThreadSize_--;
                            return;
                        }
                    }
                }
            }

            task = taskQue_.front();
            taskQue_.pop();
            idleThreadSize_--;
            curTaskQueSize_--;
            // 任务队列不满，可以提交任务
            cvTask_.notify_all();
        }

        if (task != nullptr) {
            std::cout << "线程"  << threadId << "正在执行任务。。。。" << std::endl;
            task->exec();
            idleThreadSize_++;
            lastIdleTime = std::chrono::system_clock::now();
        }
    }
}

Result ThreadPool:: submitTask(std::shared_ptr<Task> task) {
    std::unique_lock<std::mutex> lck(mtx_);

    while (curTaskQueSize_ > maxTaskSize_) {
        std::cv_status flag = cvTask_.wait_for(lck,std::chrono::seconds(maxSubmitTaskTime_));
        if (flag == std::cv_status::timeout) {
            if (curTaskQueSize_ > maxTaskSize_) {
                LOG("Submit false ! ! !");
                return Result(task,false);
            }
        }// else继续while判断
    }

    taskQue_.push(task);
    ++curTaskQueSize_;

    // 有任务，可以运行了
    cvThread_.notify_all();

    // cached模式
    if (poolMode_ == PoolMode::CACHED && curThreadSize_ < maxThreadSize_ && idleThreadSize_ < curTaskQueSize_) {
        // 创建线程
        auto threadPtr = std::make_shared<Thread>(std::bind(&ThreadPool::threadHandler,this,std::placeholders::_1));
        int threadId = threadPtr->getThreadId();
        threadMap_[threadId] = threadPtr;
        ++curThreadSize_;
        idleThreadSize_++;

        std::cout << "新增线程"  << threadId << "。。。。" << std::endl;

        threadMap_[threadId]->start();
    }

    return Result(task,true);
}