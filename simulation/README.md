# UbiBot IoT 设备仿真程序 (simulation/)

一个用嵌入式 C 风格编写的 IoT 设备仿真器：在 Windows / Linux 主机上编译运行，模拟一台
真实的 UbiBot 传感设备与云端（`server/`，协议见 `docs/UbiBot开放平台硬件通信协议.md`）
对话——设备只用 pid+sn 明文标识自己（无密钥、无签名、无激活步骤），周期性采集 `field1`
(温度)/`field2`(湿度)/`field3`(光照) 三个内置传感器，批量上报给服务器；服务器首次收到某个
sn 的上报时自动创建设备记录。

**这份 C 代码本身就是可以直接用于真实 FreeRTOS 固件的产品代码**，不是"仿真专用"的示例代码。
详见下方"目录结构"里的分层说明，以及 [`freertos_port/README.md`](freertos_port/README.md)。

## 快速开始

```bash
# 方式一：CMake（推荐，Windows/Linux 通用）
cmake -S . -B build
cmake --build build
ctest --test-dir build          # 跑纯逻辑单测（不需要真实服务器）

# 方式二：Makefile（gcc / mingw-w64 均可）
make            # 编译 ub_device_sim 并跑单测
```

Windows 下用 MinGW：

```bash
cmake -S . -B build -G "MinGW Makefiles"
cmake --build build
```

或直接用 mingw-w64 交叉编译（Linux 上生成 Windows exe）：

```bash
x86_64-w64-mingw32-gcc -std=c11 -Icore/include -Itransport/include -Iapp/include \
  -o ub_device_sim.exe app/src/main.c core/src/ub_json.c core/src/ub_protocol.c \
  transport/src/ub_transport_sockets.c app/src/ub_platform_host.c \
  app/src/ub_sensors_sim.c app/src/ub_device.c -lws2_32
```

运行仿真设备（默认参数对应 `cmd/server/main.go` 里预置的演示设备，本机起一个
`go run ./cmd/server` 即可直接联调；由于协议不再需要预先创建/激活设备，换一个新的
`--sn` 也能直接用，服务器会在第一次收到上报时自动建档）：

```bash
./build/ub_device_sim --host 127.0.0.1 --port 8080
```

可选参数：`--pid` `--sn` `--tick-ms`（主循环心跳间隔，默认 1000ms）。`Ctrl+C` 结束。

没有 `--secret`（协议里已经没有密钥/签名这回事），也没有 `--data-dir`（见下方"目录结构"里
关于状态持久化的说明——这个协议下设备没有任何值得在重启后恢复的状态）。

## 目录结构与分层

```
simulation/
├── core/           零 OS 依赖：协议编解码，任何平台不用改一行代码
│   ├── include/    ub_json.h ub_protocol.h
│   └── src/        对应实现，全部定长缓冲区、无 malloc
├── transport/      HTTP 收发接口的定义 + 仅用于主机联调的 socket 实现
│   ├── include/    ub_transport.h（一个函数指针：post）
│   └── src/        ub_transport_sockets.c —— 唯一"仅主机可用"的文件，
│                   FreeRTOS 上要换成 lwIP/厂商 TLS 栈实现
├── app/            设备状态机 + HAL 头文件的主机实现
│   ├── include/    ub_platform.h ub_sensors.h ub_device.h
│   └── src/        ub_device.c（状态机，跨平台无关）+ ub_platform_host.c /
│                   ub_sensors_sim.c（主机侧 HAL 实现，FreeRTOS 上分别替换）
├── test/           单测 + （需要真实服务器的）集成测试 + 手动联调用的 mock_server.py
├── freertos_port/  FreeRTOS 移植说明与参考骨架（未编译，仅作映射示例）
├── CMakeLists.txt
└── Makefile
```

也就是说，**只有 `transport/src/ub_transport_sockets.c` 和 `app/src/ub_platform_host.c` /
`ub_sensors_sim.c` 这三个文件是"主机专用"的**；`core/` 全部、`app/src/ub_device.c`
（状态机本体）、以及 `app/include/*.h` 里定义的接口，原样搬到 FreeRTOS 工程里就能用。移植时
只需要给 `ub_platform.h` / `ub_sensors.h` / `ub_transport.h` 各写一份 FreeRTOS 侧实现文件，
替换掉这三个 `_host`/`_sim`/`_sockets` 文件。

这份协议没有会话 Token、没有服务端下发的配置/指令通道，因此设备端**没有任何需要持久化的
状态**（原来的 `ub_storage.h` 持久化层——保存 token/探头表/未完成的 ack 队列——已经随着这些
功能一起整体移除）：`ub_device_init()` 每次都是从 pid/sn/固定采样与上报周期这几个已知量重新
初始化，一次全新的进程和"重启后恢复"没有任何区别。

## 已验证过的行为（本仓库内实测，非纸面推断）

- Linux 原生 gcc 编译：`ub_device_sim` + `test_protocol`（47/47）全部零警告通过。
- 用一个最小 mock HTTP 服务器（`test/mock_server.py`，仅用于手动联调，不参与构建）实测过
  完整设备生命周期：无需激活，直接周期采样 → 上报 `POST /api/v1/data/report`（服务器自动
  建档）→ 收到 `{"c":0,"t":...}` 后清空已上报的缓冲、校准本地时钟。
- 关掉服务器/网络请求失败时验证过设备会保留已采样但未上报的数据、下一轮心跳重试，不会
  丢数据也不会崩溃（受限于本地采样缓冲区容量，见下方"已知的仿真简化点"）。
- 时间戳校验（1002）路径：验证过设备收到该错误码后会主动调用 `/auth/time` 校准本地时钟，
  再在下一个上报周期用修正后的时间戳重试。

## 与真实协议的对应关系

见 `docs/UbiBot开放平台硬件通信协议.md`：§3 时间同步（`ub_device.c` 里的
`do_time_sync()`，仅在收到 1002 时间戳错误后主动调用一次，不是启动必经步骤）、§4 数据上报
（`do_report()`，`ub_device_tick()` 里的采样/上报周期定时器）、§5 数据字段（`sample()` 只
使用 `field1`/`field2`/`field3`，对应 `ub_sensors.h` 的
`ub_sensor_read_temperature`/`_humidity`/`_light`）、§7 错误处理（`do_report()` 里对
`c` 非 0 时的分支：1002 触发时间重同步、1103 记录持续告警、其余错误保留缓冲区等下个周期
重试）。

## 已知的仿真简化点（如实标注，不是缺陷）

- `ub_sensors_sim.c` 是随机游走出来的假数据，不接真实传感器；`field3`(光照) 的量程选了
  0–100000 lux（覆盖夜间到直射阳光的常见范围），起始基线 20000 lux，每步最大变化 800 lux。
- 本地采样缓冲区固定 16 条（`UB_SAMPLE_BUF_CAP`），采满后新采样会被丢弃直到下一次成功上报
  清空缓冲区——真实固件可能需要更大的缓冲区或落盘队列来应对更长的离线窗口，这里为了简单没有
  做无限增长的队列。
- 采样/上报周期（默认 30s / 300s）是写死在 `ub_device.c` 里的常量，不是 CLI 参数，也没有
  服务端下发配置的通道去改它们——协议本身已经不提供这个能力。
- `ub_transport_sockets.c` 不支持 TLS（`https://` 会被当作 `http://` 处理），仅用于本地/
  内网联调；真实固件的传输层通常直接对接厂商已有的 TLS 支持。
- 单个设备进程内所有 HTTP 调用都是同步阻塞的（符合大多数单任务/单线程嵌入式设备的现实
  情况）。
