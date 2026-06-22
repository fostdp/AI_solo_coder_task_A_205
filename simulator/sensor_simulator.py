#!/usr/bin/env python3
"""
古代海船纵帆传感器模拟器
模拟每面帆每1分钟通过UDP上报风速、风向、帆角、航速数据
"""

import json
import socket
import time
import random
import math
import argparse
import threading
from datetime import datetime, timezone
from dataclasses import dataclass, asdict, field
from typing import List, Optional

try:
    import paho.mqtt.client as mqtt
    MQTT_AVAILABLE = True
except ImportError:
    MQTT_AVAILABLE = False


@dataclass
class SailConfig:
    sail_id: int
    sail_name: str
    sail_position: str
    area: float
    aspect_ratio: float
    camber: float


@dataclass
class ShipConfig:
    ship_id: int
    ship_name: str
    hull_length: float
    hull_width: float
    sails: List[SailConfig]
    base_heading: float = 0.0
    current_heading: float = 0.0
    base_speed: float = 3.0


@dataclass
class SensorReading:
    time: str
    ship_id: int
    sail_id: int
    wind_speed: float
    wind_direction: float
    sail_angle: float
    ship_speed: float
    heading: float
    ambient_temp: float
    air_density: float


SHIP_CONFIGS = [
    ShipConfig(
        ship_id=1,
        ship_name="南海一号",
        hull_length=30.5,
        hull_width=9.8,
        sails=[
            SailConfig(1, "主桅帆", "主帆", 150.0, 2.8, 0.13),
            SailConfig(2, "前桅帆", "前帆", 100.0, 2.5, 0.12),
            SailConfig(3, "后桅帆", "尾帆", 80.0, 2.3, 0.11),
        ],
        base_heading=45.0,
        base_speed=3.5,
    ),
    ShipConfig(
        ship_id=2,
        ship_name="泉舶",
        hull_length=26.0,
        hull_width=8.2,
        sails=[
            SailConfig(4, "主桅帆", "主帆", 120.0, 2.6, 0.12),
            SailConfig(5, "前桅帆", "前帆", 85.0, 2.4, 0.11),
        ],
        base_heading=60.0,
        base_speed=3.0,
    ),
    ShipConfig(
        ship_id=3,
        ship_name="广舶",
        hull_length=33.0,
        hull_width=10.5,
        sails=[
            SailConfig(6, "主桅帆", "主帆", 180.0, 2.9, 0.14),
            SailConfig(7, "前桅帆", "前帆", 120.0, 2.6, 0.12),
            SailConfig(8, "首帆", "首斜帆", 60.0, 2.2, 0.10),
        ],
        base_heading=30.0,
        base_speed=4.0,
    ),
]


class WindSimulator:
    """模拟真实风场变化，包含日变化、阵风、渐变等"""

    def __init__(self, base_speed: float = 8.0, base_direction: float = 135.0):
        self.base_speed = base_speed
        self.base_direction = base_direction
        self.current_speed = base_speed
        self.current_direction = base_direction
        self.gust_remaining = 0
        self.gust_target = 0
        self.shear_factor = 0.0

    def update(self, dt: float) -> tuple[float, float]:
        # 风速渐变（模拟大尺度天气系统变化）
        self.base_speed += random.gauss(0, 0.05) * dt
        self.base_speed = max(2.0, min(25.0, self.base_speed))

        # 风向渐变
        self.base_direction += random.gauss(0, 0.1) * dt
        self.base_direction %= 360.0

        # 阵风模拟
        if self.gust_remaining <= 0 and random.random() < 0.02:
            self.gust_remaining = random.uniform(5, 20)
            self.gust_target = self.base_speed * random.uniform(1.3, 1.8)

        if self.gust_remaining > 0:
            self.gust_remaining -= dt
            speed_factor = min(1.0, self.gust_remaining / 20)
            self.current_speed = self.base_speed + (self.gust_target - self.base_speed) * speed_factor
        else:
            # 湍流波动
            self.current_speed = self.base_speed + random.gauss(0, 0.8)

        self.current_speed = max(0.5, self.current_speed)

        # 风向小尺度波动
        self.current_direction = self.base_direction + random.gauss(0, 3.0)
        self.current_direction %= 360.0

        return self.current_speed, self.current_direction

    def get_wind_at_height(self, height: float, base_speed: float, base_direction: float) -> tuple[float, float]:
        """风切变：风速随高度增加"""
        alpha = 0.143  # 地面风切变指数
        ref_height = 10.0
        speed_factor = (height / ref_height) ** alpha
        speed = base_speed * speed_factor
        direction = base_direction + height * 0.05  # 风向随高度轻微偏转
        return speed, direction % 360.0


class ShipMotionSimulator:
    """模拟船舶在海上的运动"""

    def __init__(self, ship_config: ShipConfig):
        self.config = ship_config
        self.heading = ship_config.base_heading
        self.speed = ship_config.base_speed
        self.roll = 0.0
        self.pitch = 0.0
        self.yaw_rate = 0.0
        self.speed_accel = 0.0

    def update(self, dt: float, wind_speed: float, wind_direction: float) -> tuple[float, float, float, float]:
        # 计算相对风向
        relative_wind = wind_direction - self.heading
        while relative_wind > 180:
            relative_wind -= 360
        while relative_wind < -180:
            relative_wind += 360

        # 简化的船舶运动学
        # 顺风加速，逆风减速
        wind_angle_rad = math.radians(relative_wind)
        wind_drive = wind_speed * math.cos(wind_angle_rad)

        target_speed = self.config.base_speed + wind_drive * 0.3
        target_speed = max(0, min(12, target_speed))

        # 速度变化
        speed_diff = target_speed - self.speed
        self.speed_accel = speed_diff * 0.05
        self.speed += self.speed_accel * dt

        # 航向变化（受侧向风力矩）
        wind_side = wind_speed * math.sin(wind_angle_rad)
        target_yaw_rate = wind_side * 0.02
        self.yaw_rate += (target_yaw_rate - self.yaw_rate) * 0.01
        self.heading += self.yaw_rate * dt
        self.heading %= 360.0

        # 横摇和纵摇
        self.roll = math.sin(time.time() * 0.3) * 3.0 + wind_side * 0.1
        self.pitch = math.cos(time.time() * 0.2) * 2.0 + wind_drive * 0.05

        return self.speed, self.heading, self.roll, self.pitch


class SailAngleController:
    """模拟帆角控制（自动调整到相对合理的角度）"""

    def __init__(self, sail_position: str):
        self.sail_position = sail_position
        self.current_angle = 30.0
        self.offset = {
            "主帆": 0.0,
            "前帆": -10.0,
            "尾帆": 15.0,
            "首斜帆": -20.0,
        }.get(sail_position, 0.0)

        self.sometimes_stall = random.random() < 0.05

    def update(self, dt: float, relative_wind: float) -> float:
        # 理想帆角约为相对风向的一半
        ideal_angle = relative_wind * 0.55 + self.offset

        # 帆角调整有滞后
        angle_diff = ideal_angle - self.current_angle
        self.current_angle += angle_diff * min(1.0, dt * 0.3)

        # 偶而出现错误调整导致失速（用于触发告警）
        if self.sometimes_stall and random.random() < 0.01:
            self.current_angle += random.choice([-25, 25])
            self.sometimes_stall = False

        return self.current_angle


class SensorSimulator:
    def __init__(self, udp_host: str, udp_port: int, interval: float = 60.0,
                 fast_mode: bool = False, debug: bool = False,
                 mqtt_broker: str = None, mqtt_topic: str = "sail/sensor",
                 wind_speed: float = None, wind_direction: float = None):
        self.udp_host = udp_host
        self.udp_port = udp_port
        self.interval = interval
        self.fast_mode = fast_mode
        self.debug = debug
        self.mqtt_broker = mqtt_broker
        self.mqtt_topic = mqtt_topic
        self.mqtt_client = None

        if self.fast_mode:
            self.interval = 1.0
            self.time_scale = 60.0
        else:
            self.time_scale = 1.0

        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.stop_event = threading.Event()

        self.wind_sims = {}
        self.ship_motions = {}
        self.sail_controllers = {}

        for ship in SHIP_CONFIGS:
            seed_bonus = ship.ship_id * 100
            random.seed(42 + seed_bonus)

            base_speed = wind_speed if wind_speed is not None else (6.0 + ship.ship_id * 1.5)
            base_dir = wind_direction if wind_direction is not None else (90.0 + ship.ship_id * 30)

            self.wind_sims[ship.ship_id] = WindSimulator(
                base_speed=base_speed,
                base_direction=base_dir,
            )
            self.ship_motions[ship.ship_id] = ShipMotionSimulator(ship)

            for sail in ship.sails:
                self.sail_controllers[(ship.ship_id, sail.sail_id)] = SailAngleController(sail.sail_position)

        random.seed(None)

        if self.mqtt_broker and MQTT_AVAILABLE:
            self.mqtt_client = mqtt.Client(client_id=f"sail-sim-{random.randint(1000,9999)}")
            self.mqtt_client.on_connect = lambda c, u, f, rc: print(f"  MQTT connected to {self.mqtt_broker}" if rc == 0 else f"  MQTT connect failed: {rc}")
            try:
                self.mqtt_client.connect(self.mqtt_broker.split(":")[-1],
                                         int(self.mqtt_broker.split(":")[-2].split("/")[-1]) if ":" in self.mqtt_broker else 1883)
                self.mqtt_client.loop_start()
            except Exception as e:
                print(f"  MQTT connection error: {e}")
                self.mqtt_client = None
        elif self.mqtt_broker and not MQTT_AVAILABLE:
            print("  Warning: paho-mqtt not installed, MQTT publishing disabled")

    def send_reading(self, reading: SensorReading):
        payload = json.dumps(asdict(reading), ensure_ascii=False).encode("utf-8")
        try:
            self.sock.sendto(payload, (self.udp_host, self.udp_port))
            if self.debug:
                print(f"[{datetime.now().strftime('%H:%M:%S')}] UDP -> Ship {reading.ship_id}/Sail {reading.sail_id}: "
                      f"Wind={reading.wind_speed:.1f}m/s Dir={reading.wind_direction:.0f}° "
                      f"Angle={reading.sail_angle:.1f}° Speed={reading.ship_speed:.2f}m/s "
                      f"({len(payload)} bytes)")
        except Exception as e:
            print(f"Error sending UDP packet: {e}")

        if self.mqtt_client:
            try:
                self.mqtt_client.publish(self.mqtt_topic, payload, qos=1)
            except Exception as e:
                print(f"Error publishing MQTT message: {e}")

    def generate_reading(self, ship: ShipConfig, sail: SailConfig,
                         wind_speed: float, wind_dir: float,
                         ship_speed: float, heading: float,
                         sail_angle: float) -> SensorReading:

        height_factor = {
            "主帆": 20.0,
            "前帆": 15.0,
            "尾帆": 12.0,
            "首斜帆": 8.0,
        }.get(sail.sail_position, 10.0)

        wind = self.wind_sims[ship.ship_id]
        local_speed, local_dir = wind.get_wind_at_height(height_factor, wind_speed, wind_dir)

        # 添加传感器噪声
        noise_level_speed = 0.05
        noise_level_angle = 0.5

        return SensorReading(
            time=datetime.now(timezone.utc).isoformat(),
            ship_id=ship.ship_id,
            sail_id=sail.sail_id,
            wind_speed=max(0, local_speed + random.gauss(0, noise_level_speed * local_speed)),
            wind_direction=(local_dir + random.gauss(0, noise_level_angle)) % 360.0,
            sail_angle=sail_angle + random.gauss(0, 0.3),
            ship_speed=max(0, ship_speed + random.gauss(0, 0.05)),
            heading=(heading + random.gauss(0, 0.2)) % 360.0,
            ambient_temp=25.0 + random.gauss(0, 0.5),
            air_density=1.225 + random.gauss(0, 0.005),
        )

    def run(self):
        print(f"🚢 海船传感器模拟器启动")
        print(f"   目标地址: {self.udp_host}:{self.udp_port}")
        print(f"   模拟船舶数: {len(SHIP_CONFIGS)}")
        print(f"   模拟风帆数: {sum(len(s.sails) for s in SHIP_CONFIGS)}")
        print(f"   上报间隔: {self.interval:.1f}秒 (快进模式: {self.fast_mode})")
        print(f"   调试输出: {self.debug}")
        print("\n船舶列表:")
        for ship in SHIP_CONFIGS:
            print(f"   - [{ship.ship_id}] {ship.ship_name}: {len(ship.sails)}面帆")
            for sail in ship.sails:
                print(f"     * [{sail.sail_id}] {sail.sail_name} ({sail.sail_position})")
        print("\n开始模拟... (Ctrl+C 停止)\n")

        last_tick = time.time()
        virtual_elapsed = 0.0
        report_count = 0

        try:
            while not self.stop_event.is_set():
                now = time.time()
                real_dt = now - last_tick
                last_tick = now

                virtual_dt = real_dt * self.time_scale
                virtual_elapsed += virtual_dt

                for ship in SHIP_CONFIGS:
                    wind = self.wind_sims[ship.ship_id]
                    wind_speed, wind_dir = wind.update(virtual_dt)

                    motion = self.ship_motions[ship.ship_id]
                    ship_speed, heading, roll, pitch = motion.update(virtual_dt, wind_speed, wind_dir)

                    for sail in ship.sails:
                        relative_wind = wind_dir - heading
                        while relative_wind > 180:
                            relative_wind -= 360
                        while relative_wind < -180:
                            relative_wind += 360

                        controller = self.sail_controllers[(ship.ship_id, sail.sail_id)]
                        sail_angle = controller.update(virtual_dt, relative_wind)

                        reading = self.generate_reading(
                            ship, sail, wind_speed, wind_dir,
                            ship_speed, heading, sail_angle
                        )
                        self.send_reading(reading)
                        report_count += 1

                if self.debug and report_count % 50 == 0:
                    print(f"\n[统计] 已发送 {report_count} 条数据, 虚拟耗时 {virtual_elapsed:.1f}s\n")

                time.sleep(self.interval)

        except KeyboardInterrupt:
            print("\n\n收到中断信号，正在停止...")
        finally:
            self.stop_event.set()
            self.sock.close()
            if self.mqtt_client:
                self.mqtt_client.loop_stop()
                self.mqtt_client.disconnect()
            print(f"✅ 模拟结束，共发送 {report_count} 条传感器数据")


def main():
    parser = argparse.ArgumentParser(
        description="古代海船纵帆传感器模拟器",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 默认UDP模式，风速8m/s风向135°
  python sensor_simulator.py --fast --wind-speed 8 --wind-direction 135

  # MQTT模式
  python sensor_simulator.py --fast --mqtt-broker tcp://mosquitto:1883

  # 只发一次用于测试
  python sensor_simulator.py --once
        """)
    parser.add_argument("--host", default="127.0.0.1", help="UDP目标主机地址 (默认: 127.0.0.1)")
    parser.add_argument("--port", type=int, default=8001, help="UDP目标端口 (默认: 8001)")
    parser.add_argument("--interval", type=float, default=60.0, help="正常模式上报间隔秒数 (默认: 60)")
    parser.add_argument("--fast", action="store_true", help="快进模式，每1秒=虚拟1分钟")
    parser.add_argument("--debug", action="store_true", help="显示详细调试信息")
    parser.add_argument("--once", action="store_true", help="只发送一次数据后退出")
    parser.add_argument("--wind-speed", type=float, default=None,
                        help="设置基础风速 m/s (默认: 每船不同 7.5/9.0/10.5)")
    parser.add_argument("--wind-direction", type=float, default=None,
                        help="设置基础风向 角度 0-360 (默认: 每船不同 120/150/180)")
    parser.add_argument("--mqtt-broker", default=None,
                        help="MQTT Broker地址，如 tcp://mosquitto:1883")
    parser.add_argument("--mqtt-topic", default="sail/sensor",
                        help="MQTT发布主题 (默认: sail/sensor)")
    parser.add_argument("--transport", choices=["udp", "mqtt", "both"], default="both",
                        help="传输方式: udp仅UDP, mqtt仅MQTT, both双发 (默认: both)")

    args = parser.parse_args()

    if args.once:
        sim = SensorSimulator(args.host, args.port, interval=0.1, debug=args.debug)
        sim.interval = 0.1
        import threading
        t = threading.Thread(target=sim.run)
        t.daemon = True
        t.start()
        time.sleep(2)
        sim.stop_event.set()
        return

    sim = SensorSimulator(
        udp_host=args.host,
        udp_port=args.port,
        interval=args.interval,
        fast_mode=args.fast,
        debug=args.debug,
        mqtt_broker=args.mqtt_broker,
        mqtt_topic=args.mqtt_topic,
        wind_speed=args.wind_speed,
        wind_direction=args.wind_direction,
    )
    sim.run()


if __name__ == "__main__":
    main()
