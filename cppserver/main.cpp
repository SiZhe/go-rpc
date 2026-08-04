#include <iostream>
#include "test/test.pb.h"
using namespace fixbug;

int main() {
    LoginRequest req;
    req.set_nama("zhangsan");
    req.set_pwd("123456");

    std::cout << "Hello, World!" << std::endl;
    return 0;
}