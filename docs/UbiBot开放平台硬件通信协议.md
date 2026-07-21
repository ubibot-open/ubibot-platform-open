# UbiBot开放平台硬件通信协议

### 1. 简介

本协议定义了UbiBot智能设备开源版与UbiBot云平台开源版通信方式。本协议设计目标是提供一套低功耗，高可靠的物联网通信协议。

本协议使用了本地凭证+动态签名+临时token认证方案。

### 2. 传输层

| 项 | 要求 |
| --- | --- |
| 协议 | HTTP 或 HTTPS 均可。生产环境建议使用 HTTPS（TLS 1.2+）；资源极度受限、或处于内网/专线环境的设备可退化为 HTTP |
| 方法 | 所有设备侧接口使用 POST（配置轮询可选接口用 GET） |
| 数据格式 | JSON，UTF-8，响应体不做美化缩进（省字节） |
| Content-Type | application/json |

注意：DeviceSecret 任何情况下都不上网络传输（无论 HTTP 还是 HTTPS）。但如果选择明文 HTTP，Token 会在链路上明文可见，存在被截获后在 Token 有效期内冒充设备上报数据的风险（无法伪造签名获取新 Token，但可以用截获的 Token 上报垃圾数据）。这个风险需要根据部署环境自行评估，协议本身不因为选择 HTTP 而降低认证强度。

### 3. 设备身份与凭证

设备出厂时烧录三元组，存储在 Flash / 安全元件中，永不明文上网：

| 字段 | 说明 |
| --- | --- |
| pid（ProductID） | 产品型号 |
| sn（SerialNumber） | 设备唯一序列号 |
| DeviceSecret | 设备密钥，仅用于本地计算签名，从不传输 |

pid、sn 本身不是机密，可明文传输。

### 4. 认证与时钟同步

因为设备初次上电时，无当前时间信息，所以，平台提供时间同步接口，设备在激活之前，如果没有同步过本地时间，或本地时钟丢失，需要先调用时间同步接口，确保本地时间戳与服务器时间戳在一个合理的误差范围内。

**时间同步：POST /api/v1/auth/time**

请求（不带时间戳）：

```json
{ "pid": "ubibot_open_dev_v1", "sn": "sn_ws1_20001_1", "sign": "3a7f...e2" }
```

其中，sign的生成方式：

sign = HEX( HMAC-SHA256(DeviceSecret, pid + sn) )

响应：

```json
{ "c": 0, "t": 1788950400, "n": "7f3a9c21" }
```

| 字段 | 说明 |
| --- | --- |
| t | 服务器当前Unix秒时间戳 |
| n | 一次性nonce，60秒内用于激活请求，用过即失效 |

**设备激活：POST /api/v1/auth/activate**

请求：

```json
{
  "pid": "ubibot_open_dev_v1",
  "sn": "sn_ws1_20001_1",
  "ts": 1788950400,
  "n": "7f3a9c21",
  "sign": "b1c4...9f"
}
```

其中：

sign = HEX( HMAC-SHA256(DeviceSecret, pid + sn + ts + n) )

n 为可选字段，分两种情况：

设备无可靠本地时钟（首次开机 / RTC 掉电重置）：先调用时间同步接口，拿到 t 和 n，ts 直接取 t，本请求带上 n。服务端校验 n 必须是刚才为这个 sn 签发、未使用过的 nonce，用后立即失效——即使 sign 被截获重放，n 也只能用一次，不依赖时间窗口即可防重放。

设备已有本地时钟（此前已从服务端同步过时间，或有电池维持的 RTC）：可以跳过 §4.1，直接用本地时间戳发起激活，n 留空，服务端退化为原方案的 ±5 分钟时间窗口校验。

注意：±5 分钟窗口本身不能防重放——同一个被截获的签名请求在窗口内可以被重复提交并各自换到一个可用 token。建议服务端在此分支额外维护每个 sn 最近一次成功激活的 ts，仅接受严格大于该值的 ts（单调递增），把窗口重放的空间收窄到"仅一次"。

```json
{ "c": 0, "token": "a2...", "exp": 86400 }
```

| 字段 | 说明 |
| --- | --- |
| token | 会话token |
| exp | token有效期（秒），默认86400 |

设备将token持久化到Flash，并记录获取时间。

错误响应：

```json
{ "c": 1002, "m": "sign mismatch" }
```

### 5. 数据上传

请求头：

*Content-Type: application/json*

*X-IoT-Token: <token>*

请求体：一次上报可携带多个时间点（例如设备离线期间缓存了多轮采集数据），每个时间点又可携带多个传感器读数：

```json
{
  "did": "ws1-20001-1",
  "recs": [
    { "ts": 1788950400, "d": { "temperature": 25.6, "humidity": 60.2 } },
    { "ts": 1788951000, "d": { "temperature": 25.8, "humidity": 59.9,
        "npk": { "n": 120, "p": 100, "k": 100 } } }
  ],
  "ack": ["c1007", "c1008"]
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| did | string | 是 | 设备ID |
| recs | array | 是 | 一条或多条记录，每条对应一个采样时间点 |
| recs[].ts | Int64 | 是 | Unix秒时间戳 |
| recs[].d | object | 是 | 传感器名->数值的映射。复合传感器（如NPK）用嵌套对象。自定义探头字段（见§7.2）按同样规则写入 |
| ack | array<string> | 否 | 确认已成功执行的历史下发指令ID，服务端收到后从待下发队列移除 |
| nak | array<object> | 否 | 确认收到但执行失败的历史下发指令，见§7.2。同一个cmd id 不会同时出现在 ack 和 nak 中 |
| prb | array<string> | 否 | 当前生效的自定义探头pid列表，仅在探头配置发生变化后的下一次上传中携带，见§7.2 |
| ota | object | 否 | 固件升级进度/状态上报，仅在有OTA任务处于非终态时携带，见§7.3 |

服务端必须校验 did 与鉴权 token 所绑定的 sn 一致，不一致时按§8的1103处理，防止持有token的一方冒充其他 did 上报数据。服务端应对 recs 的条数与单次请求体大小设置上限（例如离线缓存过多时分批上传），超限返回1003。

### 6. Token续期

服务器每次响应会通过Header:  X-Token-Expires-In 告知 Token 剩余秒数.

剩余<3600秒时，设备在下一次唤醒先执行激活，拿新Token，再上报数据。若 cfg.ui（上报间隔）被下发为大于3600秒的值，设备应改用 max(3600, 2×ui) 作为续期阈值，避免两次上报之间token提前过期。

上报若收到401，清空本地Token，重新激活后重试。

### 7. 响应与控制指令下发

设备无法被动接收服务器端推送，控制指令只能通过响应下发（数据上传接口，或 §7.1 的配置轮询接口）。

响应体结构：

```json
{
  "c": 0,
  "t": 1788950000,
  "cfg": { "ci": 30, "ui": 600, "fe": ["temperature", "humidity", "npk"] },
  "cmd": [
    { "id": "c1009", "tp": "set_cfg", "a": { "ui": 300 } },
    { "id": "c1010", "tp": "reboot" },
    { "id": "c1011", "tp": "set_probe", "a": { "op": "upsert", "probes": [
        { "pid": "p1", "key": "soil_ph", "iface": "rs485", "proto": "modbus_rtu",
          "addr": 1, "fc": 3, "reg": 0, "cnt": 1, "dtype": "u16",
          "byte_order": "abcd", "scale": 0.1, "offset": 0, "ci": 60 } ] } }
  ]
}
```

| 字段 | 说明 |
| --- | --- |
| c | 业务状态码，0成功，其他值为错误。 |
| t | 服务器时间戳，可用于提供给设备对时间进行校准 |
| cfg | 仅当设备配置自上次下发以来，发生变化时才返回此字段，否则省略。 |
| cfg.ci | 采集间隔（秒） |
| cfg.ui | 上报间隔（秒） |
| cfg.fe | 启用的传感器字段列表，省略或空=全部采集 |
| cmd | 待执行指令队列，仅当有待下发指令时出现 |
| cmd[].id | 指令的唯一ID，用于§5的ack/nak确认 |
| cmd[].tp | 指令类型：set_cfg(更新配置) / reboot(重启) / calibrate(校准) / set_probe(配置探头自定义数据读取，见§7.2) / ota(固件升级，见§7.3) 等 |
| cmd[].a | 指令参数，按tp不同而不同，可省略。 |

说明：cmd为待下发队列，同一条指令在被ack/nak确认前会持续下发；reboot等一次性动作类指令语义上视为"收到即执行成功"，设备应在真正执行重启动作之前，先在本地标记该cmd.id待ack、并尽量携带在重启前的最后一次上传里，而不是等待"重启完成"再确认（重启后设备已无法回溯确认）。

### 7.1 配置轮询（可选，GET）

**GET /api/v1/device/poll?did=ws1-20001-1**

请求头：X-IoT-Token: <token>

用于上报间隔较长、但希望更及时拿到下发指令（例如平台刚下发了set_probe/reboot）的设备主动轮询；不携带传感器数据，不产生历史记录，也不消耗cfg.ui周期。响应体结构与§7完全一致（含cfg/cmd）。该接口应单独限流（复用§8的429/1900），建议轮询频率不高于1次/分钟。

### 7.2 探头自定义数据读取配置（set_probe）

设备可能外接非固定型号的探头（RS485/Modbus、模拟量、I2C等），具体从机地址、寄存器、数据类型、换算公式因传感器而异，无法写死在固件里，需要由平台按设备下发配置。

**指令：cmd[].tp = "set_probe"**

```json
{
  "id": "c1011",
  "tp": "set_probe",
  "a": {
    "op": "upsert",
    "probes": [
      {
        "pid": "p1",
        "key": "soil_ph",
        "iface": "rs485",
        "proto": "modbus_rtu",
        "addr": 1,
        "fc": 3,
        "reg": 0,
        "cnt": 1,
        "dtype": "u16",
        "byte_order": "abcd",
        "scale": 0.1,
        "offset": 0,
        "ci": 60,
        "timeout": 500,
        "retry": 2
      }
    ]
  }
}
```

删除探头（仅需pid）：

```json
{ "id": "c1012", "tp": "set_probe", "a": { "op": "remove", "probes": [ { "pid": "p1" } ] } }
```

| 字段 | 说明 |
| --- | --- |
| a.op | upsert(按pid新增/更新) / remove(按pid删除) / replace_all(整表全量替换，用于重新全量同步) |
| probes[].pid | 探头配置的唯一标识，设备侧持久化保存，用于后续更新/删除定位。与cmd.id含义不同：cmd.id只标识"这次下发"，pid标识"这个探头配置" |
| probes[].key | 该探头读数写入§5 recs[].d 时使用的字段名，不可与内置传感器名（temperature/humidity/npk等）冲突 |
| probes[].iface | 物理接口：rs485 / i2c / analog / onewire 等，可扩展 |
| probes[].proto | 应用层协议：modbus_rtu / analog_linear / sdi12 等，可扩展 |
| probes[].addr / fc / reg / cnt | modbus_rtu专用：从机地址 / 功能码(01/02/03/04) / 寄存器起始地址 / 寄存器个数 |
| probes[].dtype | 原始数据类型：u16/i16/u32/i32/float32等 |
| probes[].byte_order | cnt>1时的字节序，modbus常见的abcd/badc/cdab/dcba |
| probes[].scale / offset | 线性换算：实际值 = 原始值 × scale + offset |
| probes[].ci | 该探头单独的采集间隔（秒），省略则跟随全局cfg.ci |
| probes[].timeout / retry | 单次读取超时(ms) / 失败重试次数，省略使用设备默认值 |

一个物理探头需要同时输出多个数值（例如同时给出温度和湿度）时，可下发多条pid不同、key不同的定义，由固件在写入d时按业务约定合并成一个嵌套对象（参考§5的npk示例）。

**执行结果确认：** set_probe 与其他cmd一样，通过下一次数据上传请求的 ack/nak 确认（见§5）。全部探头都应用成功时，cmd.id出现在ack中；只要有一个探头配置非法（如寄存器越界、iface/proto不支持），设备应将该cmd.id放入nak，并说明原因：

```json
"nak": [
  { "id": "c1011", "c": 2001, "m": "invalid register for probe p1" }
]
```

探头配置发生变化后，设备应在下一次上传中通过 prb 字段（见§5）上报当前生效的probes[].pid列表，便于平台核对下发结果，未变化时省略。

### 7.3 固件升级（OTA）

固件包体积通常远超一次cmd/report请求体的合理大小（数百KB到数MB），不适合像set_probe那样把内容内嵌在JSON里下发。OTA因此拆成两条腿：cmd只携带"去哪下载、下载什么版本、怎么校验"的元数据，固件二进制本体走独立的下载接口按需拉取；升级过程较长（下载+校验+烧录+重启可能持续数分钟到数十分钟），中间状态通过§5新增的 ota 字段随常规上报持续汇报，而不是等最终结果才响应一次。

**指令：cmd[].tp = "ota"**

```json
{
  "id": "c1013",
  "tp": "ota",
  "a": {
    "action": "start",
    "version": "1.4.2",
    "url": "/api/v1/ota/firmware?fw=fw_ws1_1.4.2",
    "size": 245760,
    "sha256": "9f2c1a...b7",
    "sig": "MEUCIQ...",
    "force": false
  }
}
```

取消一个尚未开始下载、或允许中止的升级任务（仅需version）：

```json
{ "id": "c1014", "tp": "ota", "a": { "action": "cancel", "version": "1.4.2" } }
```

| 字段 | 说明 |
| --- | --- |
| a.action | start(开始升级) / cancel(取消尚未进入不可逆阶段的升级任务) |
| a.version | 目标固件版本号，设备用于跳过"已是该版本"或"版本回退"判断 |
| a.url | 固件下载地址，可为绝对URL或相对本服务的路径（见下方下载接口）。同一固件可被多台/多次下发复用同一url |
| a.size | 固件文件总字节数，用于设备预检查可用存储空间是否足够，及下载完整性校验的辅助手段 |
| a.sha256 | 固件文件整体SHA-256（hex），下载完成后设备必须校验，不一致禁止烧录 |
| a.sig | 可选，平台对固件的签名（如ECDSA），用于防止token被截获后被用来推送恶意固件；建议生产环境强制要求，签名密钥与DeviceSecret体系独立管理 |
| a.force | 可选，默认false。true时设备应忽略"当前版本≥目标版本"的常规跳过逻辑，强制重新升级（用于修复同版本误刷的固件） |

**固件下载：GET /api/v1/ota/firmware?fw=<fw_id>**

请求头：`X-IoT-Token: <token>`；支持 `Range: bytes=<start>-` 分段/续传下载。

响应：`Content-Type: application/octet-stream`，`Content-Length`，支持Range时返回206并带`Content-Range`。设备应将下载进度（已确认写入存储的字节offset）持久化到Flash，掉电重启后凭该offset用Range续传，避免每次都从头下载。

**升级流程与状态上报：**

1. 设备收到 ota/start 指令后，不立即ack——先将 `{cmd_id, version, url, sha256}` 落盘（用于断电恢复与重启后找回上下文），再开始下载。
2. 下载/校验/烧录期间，设备在每次常规数据上传（或§7.1轮询）中携带 ota 状态，便于平台侧展示进度，不需要设备主动发起额外请求：

```json
"ota": { "id": "c1013", "version": "1.4.2", "state": "downloading", "progress": 42 }
```

| 字段 | 说明 |
| --- | --- |
| ota.id | 对应的cmd id |
| ota.version | 目标版本号 |
| ota.state | downloading / verifying / flashing / rebooting / success / failed / rolled_back |
| ota.progress | 0-100，仅downloading/flashing阶段有意义，其余阶段可省略或固定为100 |

3. 下载完成后校验sha256（及sig，若下发了该字段）：不一致则state=failed，本次OTA终止，见下方nak。
4. 校验通过后进入flashing，写入待运行分区；写入完成后设置"下次启动尝试新固件"标记并reboot。
5. 设备使用新固件启动后需完成自检（能否正常上报、关键外设是否初始化成功等）。自检通过：将该cmd_id放入下一次上报的ack，并清除断电恢复用的落盘上下文。自检失败：触发回滚（看门狗/双分区机制切回原固件），state上报为rolled_back，并将该cmd_id放入nak，附失败原因。
6. action=cancel：若设备尚处于downloading阶段（未开始flashing），可安全中止并ack该cancel指令本身；若已进入flashing及之后阶段，视为不可逆，应nak该cancel指令并继续原升级流程，不允许"边写边撤"造成砖机。

**Nak 原因码（示例，与全局业务码§8独立，仅描述cmd级失败原因）：**

| c | 说明 |
| --- | --- |
| 2101 | 固件下载失败或超时（多次重试后仍失败） |
| 2102 | 设备可用存储空间不足，无法容纳固件 |
| 2103 | SHA-256校验不一致，固件文件不完整或被篡改 |
| 2104 | 签名校验失败（下发了a.sig但验签不通过） |
| 2105 | 烧录失败（Flash写入错误） |
| 2106 | 新固件自检失败，已回滚到原固件 |
| 2107 | 目标版本号非法（低于当前版本且force非true，或版本格式不识别） |

### 8. 错误处理

| HTTP状态 | c | 场景 | 设备行为 |
| --- | --- | --- | --- |
| 200 | 0 | 成功 | 正常处理 |
| 400 | 1002 | 签名校验失败/nonce无效或已用/时间戳超时 | 重新获取时间 |
| 400 | 1003 | 请求体格式错误 | 检查固件序列化逻辑，下个周期重试 |
| 401 | 1101 | Token失效 | 清空本地Token，重新激活，获取Token |
| 401 | 1102 | Token过期 | 清空本地Token，重新激活，获取Token |
| 401 | 1103 | 设备不存在、did与token绑定的sn不一致、或设备已被平台禁用 | 清空本地Token与did缓存，停止重试并告警（LED/日志），需人工核实sn/pid或售后介入 |
| 429 | 1900 | 触发限流 | 放弃本次上报，等待下个周期 |
| 5xx | 5000 | 服务端故障 | 等待下个上传周期重试 |

本表针对整次请求级别的失败。单条下发指令（cmd）执行失败不算请求失败，请求仍返回c=0，指令级失败通过§5的nak字段逐条上报，见§7.2、§7.3示例。
