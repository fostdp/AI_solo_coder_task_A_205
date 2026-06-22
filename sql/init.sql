-- TimescaleDB 初始化脚本
-- 古代海船纵帆空气动力学仿真数据库

CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 船舶信息表
CREATE TABLE IF NOT EXISTS ships (
    ship_id SERIAL PRIMARY KEY,
    ship_name VARCHAR(100) NOT NULL,
    ship_type VARCHAR(50) DEFAULT '宋代海船',
    hull_length DECIMAL(10,2) DEFAULT 30.0,
    hull_width DECIMAL(10,2) DEFAULT 8.0,
    displacement DECIMAL(12,2) DEFAULT 500.0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 风帆信息表
CREATE TABLE IF NOT EXISTS sails (
    sail_id SERIAL PRIMARY KEY,
    ship_id INTEGER NOT NULL REFERENCES ships(ship_id),
    sail_name VARCHAR(50) NOT NULL,
    sail_position VARCHAR(20) DEFAULT '主帆',
    area DECIMAL(10,2) NOT NULL DEFAULT 120.0,
    aspect_ratio DECIMAL(6,3) DEFAULT 2.5,
    chord_length DECIMAL(8,3) DEFAULT 7.0,
    span_length DECIMAL(8,3) DEFAULT 18.0,
    camber DECIMAL(5,3) DEFAULT 0.12,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 传感器数据表（超表）
CREATE TABLE IF NOT EXISTS sensor_data (
    time TIMESTAMPTZ NOT NULL,
    ship_id INTEGER NOT NULL REFERENCES ships(ship_id),
    sail_id INTEGER NOT NULL REFERENCES sails(sail_id),
    wind_speed DECIMAL(8,3) NOT NULL,
    wind_direction DECIMAL(7,3) NOT NULL,
    sail_angle DECIMAL(7,3) NOT NULL,
    ship_speed DECIMAL(8,3) NOT NULL,
    heading DECIMAL(7,3) DEFAULT 0,
    ambient_temp DECIMAL(6,2) DEFAULT 25.0,
    air_density DECIMAL(6,3) DEFAULT 1.225
);

SELECT create_hypertable('sensor_data', 'time',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day');

CREATE INDEX IF NOT EXISTS idx_sensor_ship_sail_time ON sensor_data (ship_id, sail_id, time DESC);

-- 空气动力学计算结果表（超表）
CREATE TABLE IF NOT EXISTS aerodynamic_results (
    time TIMESTAMPTZ NOT NULL,
    ship_id INTEGER NOT NULL REFERENCES ships(ship_id),
    sail_id INTEGER NOT NULL REFERENCES sails(sail_id),
    angle_of_attack DECIMAL(7,3) NOT NULL,
    lift_coefficient DECIMAL(8,5) NOT NULL,
    drag_coefficient DECIMAL(8,5) NOT NULL,
    lift_force DECIMAL(12,3) NOT NULL,
    drag_force DECIMAL(12,3) NOT NULL,
    pressure_drag DECIMAL(12,3) DEFAULT 0,
    friction_drag DECIMAL(12,3) DEFAULT 0,
    induced_drag DECIMAL(12,3) DEFAULT 0,
    reynolds_number DECIMAL(15,2) DEFAULT 0,
    boundary_layer_thickness DECIMAL(8,5) DEFAULT 0,
    is_stalled BOOLEAN DEFAULT FALSE,
    circulation_strength DECIMAL(10,3) DEFAULT 0,
    total_vortices INTEGER DEFAULT 0
);

SELECT create_hypertable('aerodynamic_results', 'time',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day');

CREATE INDEX IF NOT EXISTS idx_aero_ship_sail_time ON aerodynamic_results (ship_id, sail_id, time DESC);

-- 帆角优化结果表（超表）
CREATE TABLE IF NOT EXISTS optimization_results (
    time TIMESTAMPTZ NOT NULL,
    ship_id INTEGER NOT NULL REFERENCES ships(ship_id),
    sail_id INTEGER NOT NULL REFERENCES sails(sail_id),
    initial_sail_angle DECIMAL(7,3) NOT NULL,
    optimal_sail_angle DECIMAL(7,3) NOT NULL,
    initial_ship_speed DECIMAL(8,3) NOT NULL,
    optimized_ship_speed DECIMAL(8,3) NOT NULL,
    speed_increase DECIMAL(8,4) NOT NULL,
    wind_speed_used DECIMAL(8,3) NOT NULL,
    wind_direction_used DECIMAL(7,3) NOT NULL,
    iterations INTEGER NOT NULL,
    convergence_threshold DECIMAL(10,6) NOT NULL,
    hull_drag_initial DECIMAL(12,3) DEFAULT 0,
    hull_drag_optimized DECIMAL(12,3) DEFAULT 0,
    net_thrust_initial DECIMAL(12,3) DEFAULT 0,
    net_thrust_optimized DECIMAL(12,3) DEFAULT 0
);

SELECT create_hypertable('optimization_results', 'time',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day');

-- 告警事件表（超表）
CREATE TABLE IF NOT EXISTS alert_events (
    time TIMESTAMPTZ NOT NULL,
    ship_id INTEGER NOT NULL REFERENCES ships(ship_id),
    sail_id INTEGER REFERENCES sails(sail_id),
    alert_type VARCHAR(30) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    message TEXT NOT NULL,
    current_value DECIMAL(12,4),
    threshold_value DECIMAL(12,4),
    acknowledged BOOLEAN DEFAULT FALSE,
    resolved BOOLEAN DEFAULT FALSE
);

SELECT create_hypertable('alert_events', 'time',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day');

CREATE INDEX IF NOT EXISTS idx_alerts_severity_time ON alert_events (severity, time DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_unresolved ON alert_events (resolved, time DESC);

-- 极曲线数据表
CREATE TABLE IF NOT EXISTS polar_curves (
    polar_id SERIAL PRIMARY KEY,
    sail_id INTEGER NOT NULL REFERENCES sails(sail_id),
    wind_speed DECIMAL(8,3) NOT NULL,
    angle_of_attack DECIMAL(7,3) NOT NULL,
    lift_coefficient DECIMAL(8,5) NOT NULL,
    drag_coefficient DECIMAL(8,5) NOT NULL,
    lift_to_drag_ratio DECIMAL(8,4) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_polar_sail_wind ON polar_curves (sail_id, wind_speed);

-- 连续聚合：每小时传感器数据统计
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    ship_id,
    sail_id,
    AVG(wind_speed) AS avg_wind_speed,
    MAX(wind_speed) AS max_wind_speed,
    MIN(wind_speed) AS min_wind_speed,
    AVG(wind_direction) AS avg_wind_direction,
    AVG(sail_angle) AS avg_sail_angle,
    AVG(ship_speed) AS avg_ship_speed,
    MAX(ship_speed) AS max_ship_speed,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, ship_id, sail_id
WITH NO DATA;

-- 连续聚合：告警统计
CREATE MATERIALIZED VIEW IF NOT EXISTS alerts_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    ship_id,
    alert_type,
    severity,
    COUNT(*) AS alert_count
FROM alert_events
GROUP BY bucket, ship_id, alert_type, severity
WITH NO DATA;

-- 初始化示例船舶数据
INSERT INTO ships (ship_name, ship_type, hull_length, hull_width, displacement)
VALUES 
    ('南海一号', '宋代福船', 30.5, 9.8, 600.0),
    ('泉舶', '宋代泉舶', 26.0, 8.2, 450.0),
    ('广舶', '宋代广舶', 33.0, 10.5, 800.0)
ON CONFLICT DO NOTHING;

-- 初始化示例风帆数据
INSERT INTO sails (ship_id, sail_name, sail_position, area, aspect_ratio, chord_length, span_length, camber)
VALUES 
    (1, '主桅帆', '主帆', 150.0, 2.8, 7.3, 20.5, 0.13),
    (1, '前桅帆', '前帆', 100.0, 2.5, 6.3, 15.9, 0.12),
    (1, '后桅帆', '尾帆', 80.0, 2.3, 5.9, 13.6, 0.11),
    (2, '主桅帆', '主帆', 120.0, 2.6, 6.8, 17.6, 0.12),
    (2, '前桅帆', '前帆', 85.0, 2.4, 5.9, 14.3, 0.11),
    (3, '主桅帆', '主帆', 180.0, 2.9, 7.9, 22.8, 0.14),
    (3, '前桅帆', '前帆', 120.0, 2.6, 6.8, 17.6, 0.12),
    (3, '首帆', '首斜帆', 60.0, 2.2, 5.2, 11.5, 0.10)
ON CONFLICT DO NOTHING;

-- 告警阈值配置表
CREATE TABLE IF NOT EXISTS alert_thresholds (
    threshold_id SERIAL PRIMARY KEY,
    ship_id INTEGER REFERENCES ships(ship_id),
    parameter_name VARCHAR(50) NOT NULL,
    warning_threshold DECIMAL(12,4),
    critical_threshold DECIMAL(12,4),
    enabled BOOLEAN DEFAULT TRUE,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO alert_thresholds (ship_id, parameter_name, warning_threshold, critical_threshold)
VALUES
    (NULL, 'stall_angle', 18.0, 25.0),
    (NULL, 'min_ship_speed', 1.0, 0.5),
    (NULL, 'max_wind_speed', 20.0, 30.0)
ON CONFLICT DO NOTHING;
