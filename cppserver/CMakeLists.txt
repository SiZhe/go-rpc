cmake_minimum_required(VERSION 3.10)
project(MPRPC)

set(CMAKE_CXX_STANDARD 20)

# 可执行文件
set(EXECUTABLE_OUTPUT_PATH ${CMAKE_CURRENT_SOURCE_DIR}/bin)

# 库文件
set(LIBRARY_OUTPUT_PATH ${CMAKE_CURRENT_SOURCE_DIR}/lib)

# 头文件
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/example)
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/src/include)

# 设置项目库文件搜索路径 -L
link_directories(${PROJECT_SOURCE_DIR}/lib)

add_subdirectory(src)
add_subdirectory(example)