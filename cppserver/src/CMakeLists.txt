file(GLOB_RECURSE SRC_LIST ${CMAKE_CURRENT_SOURCE_DIR}/*.cpp ${CMAKE_CURRENT_SOURCE_DIR}/*.cc)

# 打印SRC_LIST包含的文件列表
message("SRC_LIST 包含的文件:")
foreach(file ${SRC_LIST})
    message("  ${file}")
endforeach()

add_library(mprpc ${SRC_LIST})

target_link_libraries(mprpc muduo_net muduo_base pthread zookeeper_mt)