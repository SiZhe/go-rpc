set(SRC_LIST UserService.cpp ../user.pb.cc)

add_executable(Provider ${SRC_LIST})

target_link_libraries(Provider mprpc protobuf)