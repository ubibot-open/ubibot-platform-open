# Builds the IoT device simulator and its tests. Works the same way on
# Linux (gcc/clang) and Windows (MSVC or MinGW): the only platform-specific
# code is entirely contained in transport/src/ub_transport_sockets.c and
# app/src/ub_platform_host.c, both selected by the compiler's own
# predefined _WIN32 macro, so no CMake-level platform branching is needed
# beyond linking ws2_32 on Windows.
#
# Usage:
#   cmake -S . -B build && cmake --build build
#   ctest --test-dir build              # runs the pure unit tests
#
# On Windows with MinGW:
#   cmake -S . -B build -G "MinGW Makefiles" && cmake --build build
cmake_minimum_required(VERSION 3.10)
project(ubibot_device_simulator C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

if(MSVC)
  add_compile_options(/W4)
else()
  add_compile_options(-Wall -Wextra)
endif()

# ---- core: protocol codec, zero OS dependencies ---------------------------
add_library(ubibot_core STATIC
  core/src/ub_sha256.c
  core/src/ub_hmac_sha256.c
  core/src/ub_json.c
  core/src/ub_protocol.c
)
target_include_directories(ubibot_core PUBLIC core/include)

# ---- transport: host-only sockets implementation of ub_transport.h -------
add_library(ubibot_transport STATIC
  transport/src/ub_transport_sockets.c
)
target_include_directories(ubibot_transport PUBLIC transport/include)
target_link_libraries(ubibot_transport PUBLIC ubibot_core)
if(WIN32)
  target_link_libraries(ubibot_transport PUBLIC ws2_32)
endif()

# ---- app: device state machine + host HAL implementations ----------------
add_library(ubibot_app STATIC
  app/src/ub_platform_host.c
  app/src/ub_storage_file.c
  app/src/ub_sensors_sim.c
  app/src/ub_device.c
)
target_include_directories(ubibot_app PUBLIC app/include)
target_link_libraries(ubibot_app PUBLIC ubibot_core ubibot_transport)

add_executable(ub_device_sim app/src/main.c)
target_link_libraries(ub_device_sim PRIVATE ubibot_app ubibot_transport ubibot_core)

# ---- tests -----------------------------------------------------------------
enable_testing()

add_executable(test_sha256 test/test_sha256.c)
target_link_libraries(test_sha256 PRIVATE ubibot_core)
target_include_directories(test_sha256 PRIVATE test)
add_test(NAME test_sha256 COMMAND test_sha256)

add_executable(test_protocol test/test_protocol.c)
target_link_libraries(test_protocol PRIVATE ubibot_core)
target_include_directories(test_protocol PRIVATE test)
add_test(NAME test_protocol COMMAND test_protocol)

# Not registered with ctest: needs a live server (`go run ./cmd/server`
# from the repo root). Run manually: test_integration [host] [port]
add_executable(test_integration test/test_integration.c)
target_link_libraries(test_integration PRIVATE ubibot_core ubibot_transport)
target_include_directories(test_integration PRIVATE test)
