set(SRC_LIST CallUserService.cpp ../user.pb.cc)

add_executable(Consumer ${SRC_LIST})

target_link_libraries(Consumer mprpc protobuf)