# 移植到真实 FreeRTOS 固件

`simulation/` 里除了下面这 4 个文件，其余全部（`core/` 全部 + `transport/include/
ub_transport.h` + `app/include/*.h` + `app/src/ub_device.c`，即协议编解码 + 设备状态机
本体）可以原样拷进 FreeRTOS 工程编译，不需要改一行逻辑代码：

| 主机侧文件（要替换） | 对应接口头文件 | FreeRTOS/嵌入式侧典型实现 |
|---|---|---|
| `transport/src/ub_transport_sockets.c` | `transport/include/ub_transport.h` | lwIP 的 BSD socket API（本身就兼容 `socket/connect/send/recv/close`，`ub_transport_sockets.c` 里 POSIX 分支的写法几乎可以直接照搬）或厂商 TLS/AT 指令模组封装 |
| `app/src/ub_platform_host.c` | `app/include/ub_platform.h` | `ub_platform_now/set_time` 接 RTC 驱动或 SNTP；`ub_platform_monotonic_ms` 接 `xTaskGetTickCount() * portTICK_PERIOD_MS`；`ub_platform_sleep_ms` 接 `vTaskDelay(pdMS_TO_TICKS(ms))`；`ub_platform_reboot` 接 `NVIC_SystemReset()`/`esp_restart()` 等真实复位；`ub_platform_log` 接 UART/RTT 打印 |
| `app/src/ub_storage_file.c` | `app/include/ub_storage.h` | 换成 NVS（ESP-IDF）/ 内部 flash 扇区（littlefs、自研 KV）；`ub_storage_set_base_dir` 在这类实现里通常是空操作 |
| `app/src/ub_sensors_sim.c` | `app/include/ub_sensors.h` | 换成真实 ADC/I2C/RS485-Modbus 驱动读数 |

这份目录本身**不参与编译**（没有接到 `CMakeLists.txt`/`Makefile` 里），因为它没有真实的
FreeRTOS/lwIP 头文件可编译验证——它是给移植人员看的接口映射参考，不是可运行代码。下面每个
`*_skeleton.c` 文件都是"这段代码大概长什么样"的骨架，含义与函数签名必须和
`simulation/app/include/*.h` / `simulation/transport/include/ub_transport.h` 完全一致，
里面标了 `/* TODO: */` 的地方是需要接入具体 BSP/SDK 的地方。

## 移植步骤

1. 把 `core/`、`transport/include/`、`app/include/`、`app/src/ub_device.c` 拷贝进
   FreeRTOS 工程（作为一个静态库或直接加入工程源码列表）。
2. 参照本目录的四个骨架文件，分别实现 `ub_platform.h`/`ub_storage.h`/`ub_sensors.h`/
   `ub_transport.h` 声明的函数，替换掉骨架里的 `TODO`。
3. 在 FreeRTOS 的一个任务里，写一个和 `simulation/app/src/main.c` 主循环等价的任务函数：
   `ub_device_init()` 一次，然后 `for(;;) { ub_device_tick(&dev); vTaskDelay(...); }`。
4. `ub_device_t` 结构体本身不小（含采样缓冲区、探头表等，见 `ub_device.h`），建议放在任务的
   静态/全局内存里而不是任务栈上。
5. 编译跑起来后，用 `simulation/test/mock_server.py` 或真实的 `server/` 做首次联调
   ——这两者用的是完全相同的协议编解码代码，行为应该和 `simulation/` 在主机上跑起来时一致。

## 关于 OTA 的一点提醒

`ub_device.c` 里的 `run_ota_download()` 会在下载/校验成功后直接调用
`ub_platform_reboot()`（真实固件里应触发的是"跳转到新固件并复位"，而不只是单纯复位到
当前固件——这部分通常由 bootloader/双 bank 切换逻辑负责，`ub_platform_reboot()` 只是
"请求复位"这一步，真正的固件切换需要在复位前把新固件标记为"待生效"，具体做法因目标平台
的 bootloader 方案而异，不在这份仿真代码的范围内）。`ub_device_init()` 里
`ota_pending_self_check` 分支模拟的是"复位后跑一次自检，成功则 ack、失败则
`UB_OTA_ROLLED_BACK` + nak"——真实固件要把这里的"永远成功"换成真正的启动自检结果。
