# UbiBot IoT 设备仿真程序 (simulation/)

一个用嵌入式 C 风格编写的 IoT 设备仿真器：在 Windows / Linux 主机上编译运行，模拟一台
真实的 UbiBot 传感设备与云端（`server/`，见 `docs/UbiBot开放平台硬件通信协议.md`）完整对话
——冷启动/热启动激活、周期采样上报、指令下发处理（`set_cfg`/`reboot`/`calibrate`/
`set_probe`/`ota`）、OTA 下载与断点续传、探头自定义配置、状态持久化。

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

或直接用 mingw-w64 交叉编译（Linux 上生成 Windows exe，已在本项目验证过）：

```bash
x86_64-w64-mingw32-gcc -std=c11 -Icore/include -Itransport/include -Iapp/include \
  -o ub_device_sim.exe app/src/main.c core/src/*.c transport/src/*.c app/src/ub_platform_host.c \
  app/src/ub_storage_file.c app/src/ub_sensors_sim.c app/src/ub_device.c -lws2_32
```

运行仿真设备（默认参数对应 `cmd/server/main.go` 里预置的演示设备，本机起一个
`go run ./cmd/server` 即可直接联调）：

```bash
./build/ub_device_sim --host 127.0.0.1 --port 8080
```

可选参数：`--pid` `--sn` `--secret` `--data-dir`（持久化状态存放目录）`--tick-ms`（主循环
心跳间隔，默认 1000ms）。`Ctrl+C` 结束；重新运行会从 `--data-dir` 里恢复上次的
token/配置/探头表，走热启动（本地时钟）激活路径。

## 目录结构与分层

```
simulation/
├── core/           零 OS 依赖：协议编解码 + 加密，任何平台不用改一行代码
│   ├── include/    ub_sha256.h ub_hmac_sha256.h ub_json.h ub_protocol.h
│   └── src/        对应实现，全部定长缓冲区、无 malloc
├── transport/      HTTP 收发接口的定义 + 仅用于主机联调的 socket 实现
│   ├── include/    ub_transport.h（三个函数指针：post/get/download）
│   └── src/        ub_transport_sockets.c —— 唯一"仅主机可用"的文件，
│                   FreeRTOS 上要换成 lwIP/厂商 TLS 栈实现
├── app/            设备状态机 + 四个 HAL 头文件的主机实现
│   ├── include/    ub_platform.h ub_storage.h ub_sensors.h ub_device.h
│   └── src/        ub_device.c（状态机，跨平台无关）+ 四个 *_host.c/*_sim.c
│                   （主机侧 HAL 实现，FreeRTOS 上分别替换）
├── test/           单测 + （需要真实服务器的）集成测试 + 手动联调用的 mock_server.py
├── freertos_port/  FreeRTOS 移植说明与参考骨架（未编译，仅作映射示例）
├── CMakeLists.txt
└── Makefile
```

也就是说，**只有 `transport/src/ub_transport_sockets.c` 和 `app/src/ub_platform_host.c` /
`ub_storage_file.c` / `ub_sensors_sim.c` 这四个文件是"主机专用"的**；`core/` 全部、
`app/src/ub_device.c`（状态机本体）、以及 `app/include/*.h` 里定义的接口，原样搬到
FreeRTOS 工程里就能用。移植时只需要给 `ub_platform.h` / `ub_storage.h` / `ub_sensors.h` /
`ub_transport.h` 各写一份 FreeRTOS 侧实现文件，替换掉这四个 `_host`/`_sim`/`_sockets` 文件。

## 已验证过的行为（本仓库内实测，非纸面推断）

- Linux 原生 gcc 编译：`ub_device_sim` + `test_sha256`（7/7）+ `test_protocol`（130/130）
  全部零警告通过；CMake + ctest 同样通过。
- Windows 交叉编译（mingw-w64 `x86_64-w64-mingw32-gcc`）：全部 4 个目标（仿真器 + 3 个测试
  程序）零警告生成合法 PE32+ 可执行文件。
- 用一个最小 mock HTTP 服务器（`test/mock_server.py`，仅用于手动联调，不参与构建）实测过
  完整设备生命周期：冷启动 nonce 激活 → 周期采样上报 → 收到 `set_cfg`/`set_probe` 指令并在
  下一次上报里正确 ack、探头读数出现在上报体里、`prb` 探头核对列表正确上报 → 收到 `ota`
  指令后 downloading（分块进度 0/39/78/100%）→ verifying（SHA-256 校验通过）→ flashing →
  rebooting（进程退出，模拟 MCU 重启）→ 重新启动进程后自检通过、对原始 OTA 指令下发
  `ack`、上报 `state:"success"`。
- 关掉服务器/网络请求失败时验证过设备会保留已采样但未上报的数据、下一轮心跳重试，不会
  丢数据也不会崩溃。
- 重启进程验证过状态持久化（token/exp/ci/ui/探头表/未完成的 ack 队列）能正确恢复。

## 与真实协议的对应关系

见 `docs/UbiBot开放平台硬件通信协议.md`：§4 时间同步/激活（`ub_device.c` 里 `activate()`/
`do_time_sync()`/`do_activate_with()`）、§5 数据上报（`do_report()`）、§6 Token 续期
（`ub_device_tick()` 里的 `token_remaining` 检查）、§7 指令队列与 `set_cfg`/`reboot`/
`calibrate`/`set_probe`/`ota`（`dispatch_cmds()` 及各 `apply_*()`）、§7.2 探头自定义配置
（`apply_set_probe()`，`core/include/ub_protocol.h` 里的 `ub_set_probe_args_t`）、§7.3 OTA
（`run_ota_download()`/`ota_on_chunk()`，`ub_ota_ctx_t`，`ub_transport_t.download` 的
Range 续传能力）。

## 已知的仿真简化点（如实标注，不是缺陷）

- `ub_sensors_sim.c` 是随机游走出来的假数据，不接真实传感器。
- OTA 的"自检"（`ub_device_init` 里 `ota_pending_self_check` 分支）在仿真器里永远直接
  判定成功；真实固件应替换成真正的启动自检逻辑，失败时用 `UB_OTA_ROLLED_BACK` 代替
  `UB_OTA_SUCCESS`。
- `ub_transport_sockets.c` 不支持 TLS（`https://` 会被当作 `http://` 处理），仅用于本地/
  内网联调；真实固件的传输层通常直接对接厂商已有的 TLS 支持。
- 单个设备进程内所有 HTTP 调用都是同步阻塞的（符合大多数单任务/单线程嵌入式设备的现实
  情况），因此 OTA 下载期间的周期性进度上报会阻塞主循环的采样节奏；在有 RTOS 多任务的目标
  上，通常会把 OTA 下载放到独立任务里以避免阻塞采样任务。
