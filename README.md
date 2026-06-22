# 古代海船纵帆空气动力学仿真与最佳帆角优化系统

> 宋代海船复原研究 · 涡格法(VLM) · 粘涡粒子法(VPM) · Adam梯度下降优化

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Docker Compose 编排                          │
│                                                                     │
│  ┌─────────────┐    UDP:8001     ┌──────────────────────────────┐   │
│  │   海船传感器  │ ──────────────→│                              │   │
│  │   模拟器     │                │      Go 后端服务              │   │
│  │  (Python)   │    MQTT:1883   │                              │   │
│  │             │ ────┐          │  ┌──────────┐  ┌───────────┐ │   │
│  └─────────────┘     │          │  │udp_recv  │→ │aero_sim   │ │   │
│                      │          │  └──────────┘  └───────────┘ │   │
│  ┌─────────────┐     │          │       ↓ channel    ↓          │   │
│  │   Mosquitto │←────┘          │  ┌──────────┐  ┌───────────┐ │   │
│  │  MQTT Broker│                │  │optimizer │→ │alarm_ws   │ │   │
│  │             │←───────────────│  └──────────┘  └───────────┘ │   │
│  └─────────────┘   MQTT订阅     │       ↓            ↓         │   │
│                      │          │  ┌──────────────────────────┐ │   │
│                      └──────────│  │     Orchestrator         │ │   │
│                                 │  │  (channel 多路复用)       │ │   │
│                                 │  └──────────────────────────┘ │   │
│                                 │       ↓            ↓         │   │
│                                 │  TimescaleDB    WebSocket    │   │
│                                 └──────────┬───────────────────┘   │
│                                            │                       │
│  ┌─────────────┐    HTTP:8080              │                       │
│  │   前端浏览器  │ ←────────────────────────┘                       │
│  │  Three.js   │  WebSocket /api/*                               │
│  │  Canvas     │                                                   │
│  └─────────────┘                                                   │
│                                                                     │
│  ┌─────────────┐                                                   │
│  │ TimescaleDB │  持久化存储 + 连续聚合 + 自动降采样 + 压缩        │
│  │  :5432      │                                                   │
│  └─────────────┘                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

## 数据流

```
传感器数据 → UDP/MQTT → udp_receiver/mqtt_receiver → sensorRawCh
    ↓
orchestrator → aeroReqCh → aerodynamics_simulator → aeroResCh
    ↓                                                    ↓
    ├→ optReqCh → sail_optimizer → optResCh              ↓
    ↓                         ↓                          ↓
    ├→ TimescaleDB 存储       ├→ TimescaleDB 存储         ↓
    ↓                         ↓                          ↓
    └→ checkSensorAlerts      └→ triggerStallAlert       ↓
              ↓                        ↓                 ↓
              └───────→ alertCh ──→ alarm_ws ──→ WebSocket ──→ 前端
```

## 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| 后端 | Go 1.21 | 多模块 Channel 并发架构 |
| 气动仿真 | 涡格法(VLM) + 粘涡粒子法(VPM) | 8×12=96面板 + 边界层修正 |
| 帆角优化 | Adam + Nesterov + 回溯线搜索 | 自适应学习率，风向突变快速收敛 |
| 数据库 | TimescaleDB 2.13 | 超表 + 连续聚合 + 降采样 + 压缩 |
| 消息队列 | Mosquitto MQTT 2.0 | 传感器数据双通道(UDP+MQTT) |
| 前端 | Three.js + Canvas | 3D海船模型 + 粒子流线 + 图表 |
| 监控 | Prometheus + pprof | 20+指标 + Channel积压 + 性能剖析 |
| 容器 | Docker + docker-compose | 多阶段构建，4服务编排 |

## 快速部署

### 前置条件

- Docker 20.10+
- Docker Compose v2.0+

### 一键启动

```bash
# 克隆项目
cd sail-simulation

# 启动所有服务
docker-compose up -d

# 等待服务就绪（约30秒TimescaleDB初始化）
docker-compose logs -f backend
```

服务启动后：
- 前端界面: http://localhost:8080
- Prometheus指标: http://localhost:9090/metrics
- pprof性能剖析: http://localhost:9090/debug/pprof/
- MQTT Broker: tcp://localhost:1883
- TimescaleDB: localhost:5432

### 自定义风速部署

```bash
# 设置8m/s东北风
WIND_SPEED=8 WIND_DIRECTION=45 docker-compose up -d

# 设置15m/s强风场景
WIND_SPEED=15 WIND_DIRECTION=180 docker-compose up -d
```

### 停止服务

```bash
docker-compose down

# 清除数据卷
docker-compose down -v
```

## 模拟器用法

模拟器支持 UDP 和 MQTT 双通道发送数据，可设置不同风速和风向。

### Docker 模式

```bash
# 在 docker-compose.yml 中修改 simulator 的 command
command: >
  python sensor_simulator.py
  --fast
  --host backend
  --port 8001
  --wind-speed 12
  --wind-direction 135
  --mqtt-broker tcp://mosquitto:1883
  --debug
```

### 本地模式

```bash
cd simulator

# 安装MQTT依赖（可选）
pip install paho-mqtt

# 默认模式（每60秒上报）
python sensor_simulator.py --host 127.0.0.1 --port 8001

# 快进模式（1秒=1分钟）
python sensor_simulator.py --fast --debug

# 自定义风速风向
python sensor_simulator.py --fast --wind-speed 10 --wind-direction 90

# MQTT模式
python sensor_simulator.py --fast --mqtt-broker tcp://127.0.0.1:1883

# 双通道模式（UDP + MQTT）
python sensor_simulator.py --fast --host 127.0.0.1 --mqtt-broker tcp://127.0.0.1:1883 --debug

# 只发一次数据（测试用）
python sensor_simulator.py --once
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--host` | 127.0.0.1 | UDP目标主机 |
| `--port` | 8001 | UDP目标端口 |
| `--interval` | 60 | 正常模式上报间隔(秒) |
| `--fast` | false | 快进模式，1秒=1分钟 |
| `--debug` | false | 详细调试输出 |
| `--once` | false | 只发一次后退出 |
| `--wind-speed` | 自动 | 基础风速(m/s) |
| `--wind-direction` | 自动 | 基础风向(0-360°) |
| `--mqtt-broker` | 无 | MQTT Broker地址 |
| `--mqtt-topic` | sail/sensor | MQTT发布主题 |
| `--transport` | both | 传输方式: udp/mqtt/both |

### 模拟船队

| 船舶 | 帆数 | 默认风速 | 默认风向 |
|------|------|----------|----------|
| 南海一号 | 3面 | 7.5 m/s | 120° |
| 泉舶 | 2面 | 9.0 m/s | 150° |
| 广舶 | 3面 | 10.5 m/s | 180° |

## 监控与运维

### Prometheus 指标

访问 `http://localhost:9090/metrics` 获取以下指标：

| 指标 | 说明 |
|------|------|
| `sail_sensor_received_total` | 接收传感器数据总数(按transport分) |
| `sail_sensor_validated_total` | 通过校验的传感器数据 |
| `sail_sensor_rejected_total` | 校验拒绝数(按reason分) |
| `sail_aero_simulations_total` | 气动仿真完成数 |
| `sail_optimizations_total` | 帆角优化完成数 |
| `sail_alerts_triggered_total` | 告警触发数(按type/severity分) |
| `sail_websocket_connections` | 当前WebSocket连接数 |
| `sail_channel_backlog` | Channel积压量(5秒采样) |
| `sail_lift_coefficient` | 最新升力系数 |
| `sail_drag_coefficient` | 最新阻力系数 |
| `sail_optimal_angle_degrees` | 最新最优帆角 |
| `sail_stall_status` | 失速状态(0/1) |
| `sail_db_operations_total` | 数据库操作计数 |

### pprof 性能剖析

```bash
# CPU剖析（30秒）
go tool pprof http://localhost:9090/debug/pprof/profile?seconds=30

# 内存剖析
go tool pprof http://localhost:9090/debug/pprof/heap

# Goroutine分析
go tool pprof http://localhost:9090/debug/pprof/goroutine

# 在线火焰图
# 浏览器打开 http://localhost:9090/debug/pprof/
```

### 健康检查

```bash
# API健康检查
curl http://localhost:8080/api/health

# 版本信息
curl http://localhost:8080/api/version
```

## 数据保留策略

| 数据表 | 保留期 | 压缩 | 降采样 |
|--------|--------|------|--------|
| sensor_data | 90天 | 7天后 | 5min/1hour |
| aerodynamic_results | 90天 | 7天后 | 15min |
| optimization_results | 180天 | 14天后 | 1hour |
| alert_events | 365天 | 30天后 | 1day |
| 5min降采样 | 1年 | - | - |
| 15min降采样 | 2年 | - | - |
| 小时/天聚合 | 永久 | - | - |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DATABASE_URL` | postgres://postgres:postgres@timescaledb:5432/sail_simulation | 数据库连接串 |
| `UDP_ADDR` | :8001 | UDP监听地址 |
| `HTTP_ADDR` | :8080 | HTTP监听地址 |
| `METRICS_ADDR` | :9090 | Prometheus/pprof监听地址 |
| `MQTT_BROKER` | (空=禁用) | MQTT Broker地址 |
| `MQTT_TOPIC` | sail/sensor | MQTT订阅主题 |
| `CONFIG_PATH` | ../config | 配置文件目录 |
| `FRONTEND_DIR` | ../frontend | 前端静态文件目录 |

## 项目结构

```
sail-simulation/
├── backend/                    # Go后端
│   ├── main.go                # 编排器 + HTTP服务
│   ├── go.mod / go.sum
│   ├── Dockerfile             # 多阶段构建
│   └── pkg/
│       ├── aerodynamics/      # 涡格法 + VPM
│       ├── config/            # 配置加载器
│       ├── metrics/           # Prometheus指标
│       ├── middleware/        # Gzip + CORS
│       ├── models/            # 数据模型
│       ├── modules/           # 4个独立模块
│       │   ├── udp_receiver.go
│       │   ├── mqtt_receiver.go
│       │   ├── aerodynamics_simulator.go
│       │   ├── sail_optimizer.go
│       │   ├── alarm_ws.go
│       │   └── types.go
│       ├── optimizer/         # Adam梯度下降
│       ├── storage/           # TimescaleDB
│       └── websocket/         # WebSocket Hub
├── frontend/                  # 前端
│   ├── index.html
│   ├── css/style.css
│   └── js/
│       ├── app.js            # 主入口
│       ├── junk_sail_3d.js   # Three.js 3D渲染
│       └── sail_panel.js     # UI面板+图表
├── simulator/                 # 传感器模拟器
│   ├── sensor_simulator.py
│   └── Dockerfile
├── config/                    # 外置配置
│   ├── aerodynamics.json     # 气动参数
│   └── optimizer.json        # 优化参数
├── sql/                       # 数据库
│   └── init.sql              # 建表+保留+压缩策略
├── mosquitto/                 # MQTT配置
│   └── mosquitto.conf
├── docker-compose.yml
├── .env
└── README.md
```
