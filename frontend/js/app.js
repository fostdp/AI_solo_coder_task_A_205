import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { ShipModel } from './shipModel.js';
import { AeroVisualization } from './aeroVisualization.js';
import { Charts } from './charts.js';

class App {
    constructor() {
        this.state = {
            selectedShip: 1,
            selectedSail: 1,
            windSpeed: 8,
            windDirection: 135,
            shipSpeed: 3,
            shipHeading: 45,
            sailAngles: { 1: 30, 2: 20, 3: 45, 4: 30, 5: 20, 6: 30, 7: 20, 8: 10 },
            isStalled: false,
            stallIntensity: 0,
            lastSensor: null,
            lastAero: null,
            lastOpt: null,
            alerts: [],
        };

        this.charts = new Charts();
        this.init();
    }

    init() {
        this.setupScene();
        this.setupShipModel();
        this.setupAeroViz();
        this.setupControls();
        this.setupWebSocket();
        this.setupUI();
        this.setupTabs();
        this.setupViewControls();
        this.setupShipSelector();
        this.setupSailList();
        this.startClock();
        this.loadPolarCurve();
        this.animate();
    }

    setupScene() {
        const canvas = document.getElementById('scene-canvas');
        this.canvas = canvas;

        this.scene = new THREE.Scene();
        this.scene.background = null;
        this.scene.fog = new THREE.Fog(0x0c4a6e, 60, 180);

        const rect = canvas.getBoundingClientRect();
        this.camera = new THREE.PerspectiveCamera(45, rect.width / rect.height, 0.1, 1000);
        this.camera.position.set(55, 35, 45);

        this.renderer = new THREE.WebGLRenderer({
            canvas: canvas,
            antialias: true,
            alpha: true,
        });
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
        this.renderer.setSize(rect.width, rect.height, false);
        this.renderer.shadowMap.enabled = true;
        this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;

        this.controls = new OrbitControls(this.camera, canvas);
        this.controls.enableDamping = true;
        this.controls.dampingFactor = 0.08;
        this.controls.minDistance = 15;
        this.controls.maxDistance = 150;
        this.controls.maxPolarAngle = Math.PI / 2 - 0.05;
        this.controls.target.set(0, 5, 0);

        const ambientLight = new THREE.AmbientLight(0x94a3b8, 0.6);
        this.scene.add(ambientLight);

        const sunLight = new THREE.DirectionalLight(0xfef3c7, 1.2);
        sunLight.position.set(50, 80, 30);
        sunLight.castShadow = true;
        sunLight.shadow.mapSize.width = 2048;
        sunLight.shadow.mapSize.height = 2048;
        sunLight.shadow.camera.near = 1;
        sunLight.shadow.camera.far = 200;
        sunLight.shadow.camera.left = -50;
        sunLight.shadow.camera.right = 50;
        sunLight.shadow.camera.top = 50;
        sunLight.shadow.camera.bottom = -50;
        this.scene.add(sunLight);

        const fillLight = new THREE.DirectionalLight(0x93c5fd, 0.4);
        fillLight.position.set(-40, 20, -30);
        this.scene.add(fillLight);

        const hemiLight = new THREE.HemisphereLight(0x60a5fa, 0x0e4d6c, 0.3);
        this.scene.add(hemiLight);

        this.clock = new THREE.Clock();

        window.addEventListener('resize', () => this.onResize());
    }

    setupShipModel() {
        this.shipModel = new ShipModel(this.scene, []);
        this.updateSailAngles();
    }

    setupAeroViz() {
        this.aeroViz = new AeroVisualization(this.scene, this.shipModel);
        this.aeroViz.setWind(this.state.windSpeed, this.state.windDirection);
    }

    setupControls() {
        document.getElementById('show-particles').addEventListener('change', (e) => {
            this.aeroViz.showParticles = e.target.checked;
        });
        document.getElementById('show-vortices').addEventListener('change', (e) => {
            this.aeroViz.showVortices = e.target.checked;
        });
        document.getElementById('show-streamlines').addEventListener('change', (e) => {
            this.aeroViz.showStreamlines = e.target.checked;
        });
        document.getElementById('animate-sails').addEventListener('change', (e) => {
            this.shipModel.animateSails = e.target.checked;
        });
        document.getElementById('particle-density').addEventListener('input', (e) => {
            this.aeroViz.setParticleDensity(parseInt(e.target.value));
        });

        document.getElementById('btn-optimize').addEventListener('click', () => {
            this.runManualOptimization();
        });
    }

    setupUI() {
        this.updateWindCompass();
    }

    setupTabs() {
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
                document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
                btn.classList.add('active');
                document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
            });
        });
    }

    setupViewControls() {
        document.querySelectorAll('.view-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.view-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                this.setCameraView(btn.dataset.view);
            });
        });
    }

    setCameraView(view) {
        const duration = 800;
        const startPos = this.camera.position.clone();
        const startTarget = this.controls.target.clone();
        let endPos, endTarget;

        switch (view) {
            case 'side':
                endPos = new THREE.Vector3(70, 10, 0);
                endTarget = new THREE.Vector3(0, 5, 0);
                break;
            case 'top':
                endPos = new THREE.Vector3(0, 90, 0.1);
                endTarget = new THREE.Vector3(0, 0, 0);
                break;
            case 'front':
                endPos = new THREE.Vector3(0, 15, 70);
                endTarget = new THREE.Vector3(0, 5, 0);
                break;
            case 'free':
            default:
                endPos = new THREE.Vector3(55, 35, 45);
                endTarget = new THREE.Vector3(0, 5, 0);
        }

        const startTime = performance.now();
        const animate = () => {
            const elapsed = performance.now() - startTime;
            const t = Math.min(elapsed / duration, 1);
            const ease = 1 - Math.pow(1 - t, 3);

            this.camera.position.lerpVectors(startPos, endPos, ease);
            this.controls.target.lerpVectors(startTarget, endTarget, ease);
            this.controls.update();

            if (t < 1) requestAnimationFrame(animate);
        };
        animate();
    }

    setupShipSelector() {
        document.getElementById('ship-select').addEventListener('change', (e) => {
            this.state.selectedShip = parseInt(e.target.value);
            this.loadPolarCurve();
        });
    }

    setupSailList() {
        const list = document.getElementById('sail-list');
        const sails = [
            { id: 1, ship: 1, name: '主桅帆', pos: '主帆' },
            { id: 2, ship: 1, name: '前桅帆', pos: '前帆' },
            { id: 3, ship: 1, name: '后桅帆', pos: '尾帆' },
            { id: 4, ship: 2, name: '主桅帆', pos: '主帆' },
            { id: 5, ship: 2, name: '前桅帆', pos: '前帆' },
            { id: 6, ship: 3, name: '主桅帆', pos: '主帆' },
            { id: 7, ship: 3, name: '前桅帆', pos: '前帆' },
            { id: 8, ship: 3, name: '首帆', pos: '首斜帆' },
        ];

        const shipSails = sails.filter(s => s.ship === this.state.selectedShip);
        list.innerHTML = '';
        shipSails.forEach(sail => {
            const div = document.createElement('div');
            div.className = 'sail-item' + (sail.id === this.state.selectedSail ? ' active' : '');
            div.dataset.sailId = sail.id;
            div.innerHTML = `
                <div>
                    <div class="sail-name">${sail.name}</div>
                    <div style="font-size:10px;color:#64748b;margin-top:2px">${sail.pos}</div>
                </div>
                <div class="sail-angle" id="sail-angle-${sail.id}">${(this.state.sailAngles[sail.id] || 0).toFixed(1)}°</div>
            `;
            div.addEventListener('click', () => {
                document.querySelectorAll('.sail-item').forEach(s => s.classList.remove('active'));
                div.classList.add('active');
                this.state.selectedSail = sail.id;
                this.loadPolarCurve();
            });
            list.appendChild(div);
        });

        if (shipSails.length && !shipSails.find(s => s.id === this.state.selectedSail)) {
            this.state.selectedSail = shipSails[0].id;
        }
    }

    updateSailAngles() {
        const mapping = [1, 2, 3];
        mapping.forEach((sailId, idx) => {
            this.shipModel.setSailAngle(idx, this.state.sailAngles[sailId] || 30);
        });
    }

    updateWindCompass() {
        const arrow = document.getElementById('wind-arrow');
        if (arrow) {
            arrow.setAttribute('transform', `rotate(${this.state.windDirection}, 60, 60)`);
        }
        document.getElementById('wind-speed-val').textContent = this.state.windSpeed.toFixed(1) + ' m/s';
        document.getElementById('wind-dir-val').textContent = this.state.windDirection.toFixed(0) + '°';

        const appRad = (this.state.windDirection - this.state.shipHeading) * Math.PI / 180;
        const effSpeed = Math.sqrt(
            Math.pow(this.state.windSpeed * Math.sin(appRad), 2) +
            Math.pow(this.state.windSpeed * Math.cos(appRad) - this.state.shipSpeed, 2)
        );
        document.getElementById('eff-wind-val').textContent = effSpeed.toFixed(1) + ' m/s';
    }

    setupWebSocket() {
        const wsProtocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${wsProtocol}//${location.host}/ws`;

        this.connectWS(wsUrl);
    }

    connectWS(url) {
        try {
            this.ws = new WebSocket(url);
        } catch (e) {
            this.setWSStatus(false);
            setTimeout(() => this.connectWS(url), 3000);
            return;
        }

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.setWSStatus(true);
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.setWSStatus(false);
            setTimeout(() => this.connectWS(url), 3000);
        };

        this.ws.onerror = () => {
            this.setWSStatus(false);
        };

        this.ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                this.handleWSMessage(msg);
            } catch (e) {
                console.error('WS parse error:', e);
            }
        };
    }

    setWSStatus(connected) {
        const el = document.getElementById('connection-status');
        const label = document.getElementById('ws-status');
        if (connected) {
            el.className = 'status-indicator connected';
            label.textContent = '已连接';
        } else {
            el.className = 'status-indicator disconnected';
            label.textContent = '未连接';
        }
    }

    handleWSMessage(msg) {
        switch (msg.type) {
            case 'sensor':
                this.handleSensorData(msg.data);
                break;
            case 'aerodynamic':
                this.handleAeroData(msg.data);
                break;
            case 'optimization':
                this.handleOptData(msg.data);
                break;
            case 'alert':
                this.handleAlert(msg.data);
                break;
        }
    }

    handleSensorData(data) {
        if (data.sail_id !== this.state.selectedSail) return;

        this.state.lastSensor = data;
        this.state.windSpeed = data.wind_speed;
        this.state.windDirection = data.wind_direction;
        this.state.shipSpeed = data.ship_speed;
        this.state.shipHeading = data.heading;
        this.state.sailAngles[data.sail_id] = data.sail_angle;

        document.getElementById('stat-speed').textContent = data.ship_speed.toFixed(2);
        document.getElementById(`sail-angle-${data.sail_id}`)?.replaceChildren(
            document.createTextNode(data.sail_angle.toFixed(1) + '°')
        );

        this.aeroViz.setWind(data.wind_speed, data.wind_direction);
        this.shipModel.setShipHeading(data.heading);

        const sailIdx = [1, 2, 3].indexOf(data.sail_id);
        if (sailIdx >= 0) {
            this.shipModel.setSailAngle(sailIdx, data.sail_angle);
        }

        this.updateWindCompass();
    }

    handleAeroData(data) {
        if (data.sail_id !== this.state.selectedSail) return;

        this.state.lastAero = data;
        this.state.isStalled = data.is_stalled;
        this.state.stallIntensity = data.is_stalled ?
            Math.min(1, (Math.abs(data.angle_of_attack) - 18) / 10) : 0;

        document.getElementById('stat-lift').textContent = (data.lift_force / 1000).toFixed(2);
        document.getElementById('stat-drag').textContent = (data.drag_force / 1000).toFixed(2);
        document.getElementById('stat-aoa').textContent = data.angle_of_attack.toFixed(1);

        const ld = data.drag_coefficient > 0.001 ? data.lift_coefficient / data.drag_coefficient : 0;
        document.getElementById('stat-ld').textContent = ld.toFixed(1);

        document.getElementById('info-re').textContent = data.reynolds_number.toExponential(2);
        document.getElementById('info-blt').textContent = (data.boundary_layer_thickness * 1000).toFixed(2) + ' mm';
        document.getElementById('info-pdrag').textContent = (data.pressure_drag / 1000).toFixed(2) + ' kN';
        document.getElementById('info-fdrag').textContent = (data.friction_drag / 1000).toFixed(2) + ' kN';
        document.getElementById('info-idrag').textContent = (data.induced_drag / 1000).toFixed(2) + ' kN';
        document.getElementById('info-cir').textContent = data.circulation_strength.toFixed(1);
        document.getElementById('info-vortices').textContent = data.total_vortices;

        const stallEl = document.getElementById('info-stall');
        const stallContainer = document.querySelector('.stall-indicator');
        if (data.is_stalled) {
            stallEl.textContent = '失速 ⚠';
            stallContainer.classList.add('stalled');
        } else {
            stallEl.textContent = '正常 ✓';
            stallContainer.classList.remove('stalled');
        }

        const sailItem = document.querySelector(`.sail-item[data-sail-id="${data.sail_id}"]`);
        if (sailItem) {
            sailItem.classList.toggle('stall', data.is_stalled);
        }

        this.aeroViz.setStall(this.state.stallIntensity);
        this.aeroViz.createVortexLines(data.angle_of_attack, data.lift_coefficient, data.is_stalled);

        this.charts.updateAerodynamic(data.angle_of_attack, data.lift_coefficient, data.drag_coefficient, data.is_stalled);
    }

    handleOptData(data) {
        if (data.sail_id !== this.state.selectedSail) return;

        this.state.lastOpt = data;

        document.getElementById('opt-angle-before').textContent = data.initial_sail_angle.toFixed(1);
        document.getElementById('opt-angle-after').textContent = data.optimal_sail_angle.toFixed(1);
        document.getElementById('opt-speed-before').textContent = data.initial_ship_speed.toFixed(2) + ' m/s';
        document.getElementById('opt-speed-after').textContent = data.optimized_ship_speed.toFixed(2) + ' m/s';
        document.getElementById('opt-iter').textContent = data.iterations;
        document.getElementById('opt-hulldrag').textContent = (data.hull_drag_optimized / 1000).toFixed(2) + ' kN';
        document.getElementById('opt-thrust').textContent = (data.net_thrust_optimized / 1000).toFixed(2) + ' kN';

        if (data.speed_increase > 0.005) {
            document.getElementById('stat-optimize-card').style.display = '';
            document.getElementById('stat-optimize').textContent = (data.speed_increase * 100).toFixed(1);
        }

        this.charts.updateOptimization(data);
    }

    handleAlert(data) {
        this.state.alerts.unshift(data);
        if (this.state.alerts.length > 100) this.state.alerts.pop();

        this.showAlertToast(data);
        this.updateAlertList();
    }

    showAlertToast(alert) {
        const container = document.getElementById('alert-toast-container') || document.body;
        const toast = document.createElement('div');
        toast.className = `alert-toast ${alert.severity}`;

        const icons = { critical: '🚨', warning: '⚠️', info: 'ℹ️' };
        const typeLabels = {
            stall: '帆面失速',
            low_speed: '航速过低',
            high_wind: '风速过高',
        };

        toast.innerHTML = `
            <div class="toast-header">
                <span class="toast-icon">${icons[alert.severity] || 'ℹ️'}</span>
                <span>${typeLabels[alert.alert_type] || alert.alert_type}</span>
            </div>
            <div class="toast-message">${alert.message}</div>
        `;

        container.appendChild(toast);

        setTimeout(() => {
            toast.style.animation = 'toast-out 0.3s ease forwards';
            setTimeout(() => toast.remove(), 300);
        }, 5000);
    }

    updateAlertList() {
        const list = document.getElementById('alert-list');
        if (!list) return;

        const critical = this.state.alerts.filter(a => a.severity === 'critical').length;
        const warning = this.state.alerts.filter(a => a.severity === 'warning').length;
        const info = this.state.alerts.filter(a => a.severity === 'info').length;

        const summary = document.getElementById('alert-summary');
        if (summary) {
            summary.innerHTML = `
                <div class="alert-summary-item critical">
                    <div class="alert-count">${critical}</div>
                    <div class="alert-label">严重告警</div>
                </div>
                <div class="alert-summary-item warning">
                    <div class="alert-count">${warning}</div>
                    <div class="alert-label">警告</div>
                </div>
                <div class="alert-summary-item info">
                    <div class="alert-count">${info}</div>
                    <div class="alert-label">提示</div>
                </div>
            `;
        }

        const typeLabels = {
            stall: '帆面失速',
            low_speed: '航速过低',
            high_wind: '风速过高',
        };

        list.innerHTML = this.state.alerts.slice(0, 30).map(a => {
            const time = new Date(a.time).toLocaleTimeString('zh-CN');
            const valueText = a.current_value != null && a.threshold_value != null
                ? `当前: ${Number(a.current_value).toFixed(2)} / 阈值: ${Number(a.threshold_value).toFixed(2)}`
                : '';
            return `
                <div class="alert-item ${a.severity}">
                    <div class="alert-content">
                        <div class="alert-type">${typeLabels[a.alert_type] || a.alert_type}</div>
                        <div class="alert-message">${a.message}</div>
                        ${valueText ? `<div class="alert-value">${valueText}</div>` : ''}
                    </div>
                    <div class="alert-time">${time}</div>
                </div>
            `;
        }).join('');
    }

    startClock() {
        const update = () => {
            const el = document.getElementById('clock');
            if (el) {
                const now = new Date();
                el.textContent = now.toLocaleString('zh-CN', {
                    year: 'numeric',
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                });
            }
        };
        update();
        setInterval(update, 1000);
    }

    async loadPolarCurve() {
        try {
            const windSpeed = Math.round(this.state.windSpeed * 2) / 2;
            const url = `/api/polar-curve?sail_id=${this.state.selectedSail}&wind_speed=${windSpeed}`;
            const res = await fetch(url);
            if (res.ok) {
                const data = await res.json();
                this.charts.updatePolarCurve(data);
            }
        } catch (e) {
            const defaultPolars = [];
            for (let aoa = -20; aoa <= 30; aoa += 1) {
                const aoaRad = aoa * Math.PI / 180;
                const cl0 = 2 * Math.PI * 0.12;
                const clAlpha = 2 * Math.PI * 2.8 / (2.8 + 2);
                let cl = cl0 + clAlpha * aoaRad;
                let cd = 0.009 + (cl * cl) / (Math.PI * 2.8 * 0.95);

                if (Math.abs(aoa) > 18) {
                    const excess = Math.abs(aoa) - 18;
                    cl *= Math.exp(-0.08 * excess);
                    cd *= (1 + 0.15 * excess);
                }

                defaultPolars.push({
                    angle_of_attack: aoa,
                    lift_coefficient: cl,
                    drag_coefficient: cd,
                    lift_to_drag_ratio: cd > 0.001 ? cl / cd : 0,
                });
            }
            this.charts.updatePolarCurve(defaultPolars);
        }
    }

    async runManualOptimization() {
        if (!this.state.lastSensor) {
            alert('请等待传感器数据...');
            return;
        }
        const btn = document.getElementById('btn-optimize');
        btn.disabled = true;
        btn.textContent = '优化中...';

        try {
            const res = await fetch('/api/optimize', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    ship_id: this.state.selectedShip,
                    sail_id: this.state.selectedSail,
                    wind_speed: this.state.windSpeed,
                    wind_direction: this.state.windDirection,
                    sail_angle: this.state.sailAngles[this.state.selectedSail],
                    ship_speed: this.state.shipSpeed,
                    heading: this.state.shipHeading,
                }),
            });
            if (res.ok) {
                const data = await res.json();
                this.handleOptData(data);
            }
        } catch (e) {
            console.error('Optimization failed:', e);
        } finally {
            btn.disabled = false;
            btn.textContent = '🔧 立即优化帆角';
        }
    }

    onResize() {
        const rect = this.canvas.getBoundingClientRect();
        this.camera.aspect = rect.width / rect.height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(rect.width, rect.height, false);
    }

    animate() {
        requestAnimationFrame(() => this.animate());

        const delta = Math.min(this.clock.getDelta(), 0.1);

        this.controls.update();

        const relWindAngle = this.state.windDirection - this.state.shipHeading;
        this.shipModel.update(delta, this.state.windSpeed, this.state.windDirection, relWindAngle);

        this.aeroViz.update(delta);

        this.renderer.render(this.scene, this.camera);
    }
}

window.addEventListener('DOMContentLoaded', () => {
    window.app = new App();
});
