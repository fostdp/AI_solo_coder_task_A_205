import { ShipModel, AeroVisualization, setup3DScene } from './junk_sail_3d.js';
import { SailPanel } from './sail_panel.js';

const state = {
    ws: null,
    shipModel: null,
    aeroViz: null,
    panel: null,
    scene: null,
    camera: null,
    renderer: null,
    controls: null,
    clock: null,
    currentShipID: 1,
    currentSailID: 1,
    windSpeed: 5,
    windDirection: 0,
    sailAngle: 0,
    isStalled: false,
    alerts: []
};

function init() {
    const container = document.getElementById('canvas-container');
    if (!container) {
        console.error('canvas-container not found');
        return;
    }

    const { scene, camera, renderer, controls } = setup3DScene(container);
    state.scene = scene;
    state.camera = camera;
    state.renderer = renderer;
    state.controls = controls;
    state.clock = new THREE.Clock();

    state.shipModel = new ShipModel(scene);
    state.aeroViz = new AeroVisualization(scene, { particleCount: 200, streamlineCount: 18 });
    state.aeroViz.setWind(5, 0);

    state.panel = new SailPanel();
    state.panel.init();

    state.shipModel.updateSailShape(0, 0, 0.13);
    state.shipModel.updateSailShape(1, 0, 0.13);
    state.shipModel.updateSailShape(2, 0, 0.13);

    state.panel.loadPolarCurve(1, 10);

    initWebSocket();
    setupEventListeners();
    animate();

    console.log('Sail Simulation App initialized');
}

function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    state.ws = new WebSocket(wsUrl);

    state.ws.onopen = () => {
        console.log('WebSocket connected');
        addStatusMessage('WebSocket连接成功', 'success');
    };

    state.ws.onclose = () => {
        console.log('WebSocket disconnected');
        addStatusMessage('WebSocket已断开，3秒后重试...', 'error');
        setTimeout(initWebSocket, 3000);
    };

    state.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        addStatusMessage('WebSocket连接错误', 'error');
    };

    state.ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            handleWebSocketMessage(data);
        } catch (e) {
            console.error('Failed to parse WS message:', e);
        }
    };
}

function handleWebSocketMessage(data) {
    switch (data.type) {
        case 'sensor':
            handleSensorData(data.data);
            break;
        case 'aerodynamic':
            handleAeroData(data.data);
            break;
        case 'optimization':
            handleOptData(data.data);
            break;
        case 'alert':
            state.panel.handleAlert(data.data);
            break;
        default:
            console.log('Unknown message type:', data.type);
    }
}

function handleSensorData(data) {
    state.currentShipID = data.ship_id;
    state.currentSailID = data.sail_id;
    state.windSpeed = data.wind_speed;
    state.windDirection = data.wind_direction;
    state.sailAngle = data.sail_angle;

    state.panel.updateSensorData(data);
    state.aeroViz.setWind(data.wind_speed, data.wind_direction);

    const sailIdx = (data.sail_id - 1) % 3;
    state.shipModel.updateSailShape(sailIdx, data.sail_angle, 0.13);
    state.panel.updateSailAngle(sailIdx, data.sail_angle);
}

function handleAeroData(data) {
    state.panel.updateAeroData(data);

    if (data.is_stalled !== state.isStalled) {
        state.isStalled = data.is_stalled;
        state.aeroViz.setStall(data.is_stalled);
    }
}

function handleOptData(data) {
    state.panel.updateOptData(data);
}

function setupEventListeners() {
    window.addEventListener('resize', onWindowResize);

    document.getElementById('btn-optimize')?.addEventListener('click', async () => {
        const req = {
            ship_id: state.currentShipID,
            sail_id: state.currentSailID,
            wind_speed: state.windSpeed,
            wind_direction: state.windDirection,
            sail_angle: state.sailAngle,
            ship_speed: parseFloat(document.getElementById('stat-speed')?.textContent || '0'),
            heading: 0
        };
        const result = await state.panel.optimizeSail(req);
        if (result) {
            addStatusMessage(`优化完成: ${result.efficiency_gain.toFixed(1)}% 增益`, 'success');
        }
    });

    document.getElementById('show-particles')?.addEventListener('change', (e) => {
        state.aeroViz.toggleParticles(e.target.checked);
    });

    document.getElementById('show-streamlines')?.addEventListener('change', (e) => {
        state.aeroViz.toggleStreamlines(e.target.checked);
    });

    document.getElementById('show-vortices')?.addEventListener('change', (e) => {
        state.aeroViz.toggleVortexLines(e.target.checked);
    });

    document.getElementById('animate-sails')?.addEventListener('change', (e) => {
        state.shipModel.setSailAnimation(e.target.checked);
    });

    document.getElementById('particle-density')?.addEventListener('input', (e) => {
        state.aeroViz.setParticleCount(parseInt(e.target.value));
    });
}

function onWindowResize() {
    const container = document.getElementById('canvas-container');
    if (!container || !state.camera || !state.renderer) return;
    state.camera.aspect = container.clientWidth / container.clientHeight;
    state.camera.updateProjectionMatrix();
    state.renderer.setSize(container.clientWidth, container.clientHeight);
}

function animate() {
    requestAnimationFrame(animate);

    const deltaTime = state.clock.getDelta();

    state.controls.update();

    state.shipModel.update(deltaTime, state.windSpeed, state.windDirection);
    state.aeroViz.update(deltaTime);

    state.renderer.render(state.scene, state.camera);
}

function addStatusMessage(message, type = 'info') {
    const container = document.getElementById('alert-toast-container');
    if (!container) return;

    const msg = document.createElement('div');
    msg.className = `toast status-${type}`;
    msg.innerHTML = `
        <div class="toast-title">系统消息</div>
        <div class="toast-message">${message}</div>
    `;
    container.appendChild(msg);
    requestAnimationFrame(() => msg.classList.add('show'));

    setTimeout(() => {
        msg.classList.remove('show');
        setTimeout(() => msg.remove(), 300);
    }, 5000);

    while (container.children.length > 10) {
        container.removeChild(container.firstChild);
    }
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
} else {
    init();
}

export { state };
