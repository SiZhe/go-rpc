#pragma once
#include <queue>
#include <thread>
#include <mutex>
#include <condition_variable>

template<typename T>
class LockQueue {
public:
    void push(const T &data) {
        std::unique_lock<std::mutex> lck(mtx_);
        queue_.push(data);
        cv_.notify_all();
    }

    T pop() {
        std::unique_lock<std::mutex> lck(mtx_);
        while (queue_.empty()) {
            cv_.wait(lck); // 进入wait状态 并释放锁
        }

        T data = queue_.front();
        queue_.pop();
        return data;
    }
private:
    std::queue<T> queue_;
    std::mutex mtx_;
    std::condition_variable cv_;
};
