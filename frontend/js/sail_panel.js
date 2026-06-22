export class Charts {
    constructor() {
        this.aeroData = {
            labels: [],
            lift: [],
            drag: [],
            angle: []
        };
        this.optData = {
            labels: [],
            current: [],
            optimal: [],
            gain: []
        };
        this.polarData = null;
        this.ldData = null;
        this.maxDataPoints = 50;
    }

    setupCanvas(canvas, width, height) {
        const dpr = window.devicePixelRatio || 1;
        canvas.width = width * dpr;
        canvas.height = height * dpr;
        canvas.style.width = width + 'px';
        canvas.style.height = height + 'px';
        const ctx = canvas.getContext('2d');
        ctx.scale(dpr, dpr);
        return ctx;
    }

    updateAerodynamic(data) {
        const time = new Date(data.time).toLocaleTimeString();
        this.aeroData.labels.push(time);
        this.aeroData.lift.push(data.lift_coefficient);
        this.aeroData.drag.push(data.drag_coefficient);
        this.aeroData.angle.push(data.angle_of_attack);

        if (this.aeroData.labels.length > this.maxDataPoints) {
            this.aeroData.labels.shift();
            this.aeroData.lift.shift();
            this.aeroData.drag.shift();
            this.aeroData.angle.shift();
        }

        const canvas = document.getElementById('chart-coefficients');
        if (canvas) this.drawCoefficients(canvas);
    }

    updateOptimization(data) {
        const time = new Date(data.time).toLocaleTimeString();
        this.optData.labels.push(time);
        this.optData.current.push(data.current_angle);
        this.optData.optimal.push(data.optimal_angle);
        this.optData.gain.push(data.efficiency_gain);

        if (this.optData.labels.length > this.maxDataPoints) {
            this.optData.labels.shift();
            this.optData.current.shift();
            this.optData.optimal.shift();
            this.optData.gain.shift();
        }

        const canvas = document.getElementById('chart-optimization');
        if (canvas) this.drawOptimization(canvas);
    }

    updatePolarCurve(data) {
        this.polarData = data;
        const canvas = document.getElementById('chart-polar');
        if (canvas) this.drawPolar(canvas);
    }

    updateLDCurve(data) {
        this.ldData = data;
        const canvas = document.getElementById('chart-ld');
        if (canvas) this.drawLD(canvas);
    }

    drawGrid(ctx, width, height, padding) {
        ctx.strokeStyle = 'rgba(100, 150, 200, 0.15)';
        ctx.lineWidth = 1;

        for (let i = 0; i <= 5; i++) {
            const y = padding + (height - 2 * padding) * (i / 5);
            ctx.beginPath();
            ctx.moveTo(padding, y);
            ctx.lineTo(width - padding, y);
            ctx.stroke();
        }

        for (let i = 0; i <= 5; i++) {
            const x = padding + (width - 2 * padding) * (i / 5);
            ctx.beginPath();
            ctx.moveTo(x, padding);
            ctx.lineTo(x, height - padding);
            ctx.stroke();
        }
    }

    drawAxes(ctx, width, height, padding, xLabel, yLabel, yMin, yMax) {
        ctx.strokeStyle = 'rgba(150, 200, 255, 0.5)';
        ctx.lineWidth = 2;

        ctx.beginPath();
        ctx.moveTo(padding, padding);
        ctx.lineTo(padding, height - padding);
        ctx.stroke();

        ctx.beginPath();
        ctx.moveTo(padding, height - padding);
        ctx.lineTo(width - padding, height - padding);
        ctx.stroke();

        ctx.fillStyle = 'rgba(150, 200, 255, 0.8)';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';

        for (let i = 0; i <= 5; i++) {
            const y = padding + (height - 2 * padding) * (i / 5);
            const val = yMax - (yMax - yMin) * (i / 5);
            ctx.fillText(val.toFixed(2), padding - 10, y + 4);
        }

        ctx.fillStyle = 'rgba(150, 200, 255, 0.8)';
        ctx.font = 'bold 12px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(xLabel, width / 2, height - 5);
        ctx.save();
        ctx.translate(12, height / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.fillText(yLabel, 0, 0);
        ctx.restore();
    }

    drawCoefficients(canvas) {
        const width = canvas.clientWidth;
        const height = canvas.clientHeight;
        const ctx = this.setupCanvas(canvas, width, height);
        const padding = 45;

        ctx.clearRect(0, 0, width, height);
        this.drawGrid(ctx, width, height, padding);
        this.drawAxes(ctx, width, height, padding, '时间', '系数', -0.5, 2.0);

        if (this.aeroData.lift.length < 2) return;

        const plotWidth = width - 2 * padding;
        const plotHeight = height - 2 * padding;
        const yMin = -0.5, yMax = 2.0;
        const toY = v => padding + plotHeight * (1 - (v - yMin) / (yMax - yMin));
        const toX = i => padding + plotWidth * (i / (this.aeroData.labels.length - 1));

        ctx.strokeStyle = '#4ade80';
        ctx.lineWidth = 2;
        ctx.beginPath();
        for (let i = 0; i < this.aeroData.lift.length; i++) {
            const x = toX(i);
            const y = toY(this.aeroData.lift[i]);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();

        ctx.strokeStyle = '#f87171';
        ctx.lineWidth = 2;
        ctx.beginPath();
        for (let i = 0; i < this.aeroData.drag.length; i++) {
            const x = toX(i);
            const y = toY(this.aeroData.drag[i]);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();

        ctx.fillStyle = '#4ade80';
        ctx.font = '12px sans-serif';
        ctx.textAlign = 'left';
        ctx.fillText('Cl', width - padding - 60, padding + 15);
        ctx.fillStyle = '#f87171';
        ctx.fillText('Cd', width - padding - 60, padding + 30);
    }

    drawOptimization(canvas) {
        const width = canvas.clientWidth;
        const height = canvas.clientHeight;
        const ctx = this.setupCanvas(canvas, width, height);
        const padding = 45;

        ctx.clearRect(0, 0, width, height);
        this.drawGrid(ctx, width, height, padding);
        this.drawAxes(ctx, width, height, padding, '时间', '帆角(°) / 增益(%)', -90, 90);

        if (this.optData.current.length < 2) return;

        const plotWidth = width - 2 * padding;
        const plotHeight = height - 2 * padding;
        const yMin = -90, yMax = 90;
        const toY = v => padding + plotHeight * (1 - (v - yMin) / (yMax - yMin));
        const toX = i => padding + plotWidth * (i / (this.optData.labels.length - 1));

        ctx.strokeStyle = '#60a5fa';
        ctx.lineWidth = 2;
        ctx.beginPath();
        for (let i = 0; i < this.optData.current.length; i++) {
            const x = toX(i);
            const y = toY(this.optData.current[i]);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();

        ctx.strokeStyle = '#fbbf24';
        ctx.lineWidth = 2;
        ctx.setLineDash([5, 5]);
        ctx.beginPath();
        for (let i = 0; i < this.optData.optimal.length; i++) {
            const x = toX(i);
            const y = toY(this.optData.optimal[i]);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.fillStyle = '#60a5fa';
        ctx.font = '12px sans-serif';
        ctx.textAlign = 'left';
        ctx.fillText('当前', width - padding - 70, padding + 15);
        ctx.fillStyle = '#fbbf24';
        ctx.fillText('最优', width - padding - 70, padding + 30);
    }

    drawPolar(canvas) {
        const width = canvas.clientWidth;
        const height = canvas.clientHeight;
        const ctx = this.setupCanvas(canvas, width, height);
        const padding = 45;

        ctx.clearRect(0, 0, width, height);
        this.drawGrid(ctx, width, height, padding);
        this.drawAxes(ctx, width, height, padding, 'Cd', 'Cl', 0, 0.4);

        if (!this.polarData || this.polarData.length < 2) return;

        const plotWidth = width - 2 * padding;
        const plotHeight = height - 2 * padding;
        const cdMin = 0, cdMax = 0.4;
        const clMin = 0, clMax = 1.6;
        const toX = v => padding + plotWidth * ((v - cdMin) / (cdMax - cdMin));
        const toY = v => padding + plotHeight * (1 - (v - clMin) / (clMax - clMin));

        ctx.strokeStyle = '#c084fc';
        ctx.lineWidth = 2;
        ctx.beginPath();
        for (let i = 0; i < this.polarData.length; i++) {
            const x = toX(this.polarData[i].drag_coefficient);
            const y = toY(this.polarData[i].lift_coefficient);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();

        ctx.fillStyle = 'rgba(192, 132, 252, 0.2)';
        ctx.beginPath();
        for (let i = 0; i < this.polarData.length; i++) {
            const x = toX(this.polarData[i].drag_coefficient);
            const y = toY(this.polarData[i].lift_coefficient);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.lineTo(toX(0.4), padding + plotHeight);
        ctx.lineTo(padding, padding + plotHeight);
        ctx.closePath();
        ctx.fill();
    }

    drawLD(canvas) {
        const width = canvas.clientWidth;
        const height = canvas.clientHeight;
        const ctx = this.setupCanvas(canvas, width, height);
        const padding = 45;

        ctx.clearRect(0, 0, width, height);
        this.drawGrid(ctx, width, height, padding);
        this.drawAxes(ctx, width, height, padding, '攻角(°)', 'L/D', -20, 30);

        if (!this.polarData || this.polarData.length < 2) return;

        const plotWidth = width - 2 * padding;
        const plotHeight = height - 2 * padding;
        const aoaMin = -20, aoaMax = 30;
        const ldMin = 0, ldMax = 30;
        const toX = v => padding + plotWidth * ((v - aoaMin) / (aoaMax - aoaMin));
        const toY = v => padding + plotHeight * (1 - (v - ldMin) / (ldMax - ldMin));

        ctx.strokeStyle = '#34d399';
        ctx.lineWidth = 2;
        ctx.beginPath();
        for (let i = 0; i < this.polarData.length; i++) {
            const x = toX(this.polarData[i].angle_of_attack);
            const ld = this.polarData[i].lift_to_drag_ratio;
            const y = toY(ld);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();

        const maxIdx = this.polarData.reduce((max, d, i) => d.lift_to_drag_ratio > this.polarData[max].lift_to_drag_ratio ? i : max, 0);
        const maxX = toX(this.polarData[maxIdx].angle_of_attack);
        const maxY = toY(this.polarData[maxIdx].lift_to_drag_ratio);
        ctx.fillStyle = '#34d399';
        ctx.beginPath();
        ctx.arc(maxX, maxY, 5, 0, Math.PI * 2);
        ctx.fill();
        ctx.fillStyle = 'rgba(255, 255, 255, 0.9)';
        ctx.font = 'bold 11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(`(${this.polarData[maxIdx].angle_of_attack.toFixed(1)}°, ${this.polarData[maxIdx].lift_to_drag_ratio.toFixed(1)})`, maxX, maxY - 10);
    }
}

export class SailPanel {
    constructor() {
        this.charts = new Charts();
        this.alerts = [];
        this.maxAlerts = 100;
        this.currentTab = 'aerodynamics';
    }

    init() {
        this.setupTabs();
    }

    setupTabs() {
        const tabBtns = document.querySelectorAll('.tab-btn');
        tabBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                const tab = btn.dataset.tab;
                this.switchTab(tab);
            });
        });
    }

    switchTab(tab) {
        this.currentTab = tab;
        document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        document.querySelector(`[data-tab="${tab}"]`).classList.add('active');
        document.getElementById(`tab-${tab}`).classList.add('active');
    }

    updateSensorData(data) {
        const el = id => document.getElementById(id);
        if (el('stat-speed')) el('stat-speed').textContent = data.ship_speed.toFixed(2);
        if (el('wind-speed-val')) el('wind-speed-val').textContent = data.wind_speed.toFixed(1) + ' m/s';
        if (el('wind-dir-val')) {
            const compass = this.getCompassDirection(data.wind_direction);
            el('wind-dir-val').textContent = `${data.wind_direction.toFixed(0)}° ${compass}`;
        }
        if (el('wind-arrow')) {
            el('wind-arrow').setAttribute('transform', `rotate(${data.wind_direction}, 60, 60)`);
        }
    }

    updateAeroData(data) {
        const el = id => document.getElementById(id);
        if (el('stat-lift')) el('stat-lift').textContent = (data.lift_force / 1000).toFixed(1);
        if (el('stat-drag')) el('stat-drag').textContent = (data.drag_force / 1000).toFixed(1);
        if (el('stat-aoa')) el('stat-aoa').textContent = data.angle_of_attack.toFixed(1);
        if (el('stat-ld')) el('stat-ld').textContent = (data.lift_coefficient / Math.max(data.drag_coefficient, 0.0001)).toFixed(2);
        if (el('info-re')) el('info-re').textContent = data.reynolds_number.toExponential(2);
        if (el('info-blt')) el('info-blt').textContent = (data.boundary_layer_thickness * 1000).toFixed(2) + ' mm';
        if (el('info-pdrag')) el('info-pdrag').textContent = (data.pressure_drag / 1000).toFixed(1) + ' kN';
        if (el('info-fdrag')) el('info-fdrag').textContent = (data.friction_drag / 1000).toFixed(1) + ' kN';
        if (el('info-idrag')) el('info-idrag').textContent = (data.induced_drag / 1000).toFixed(1) + ' kN';
        if (el('info-cir')) el('info-cir').textContent = data.circulation_strength?.toFixed(2) || '-';
        if (el('info-vortices')) el('info-vortices').textContent = data.vortex_count || 96;
        if (el('info-stall')) {
            const stallEl = el('info-stall');
            if (data.is_stalled) {
                stallEl.textContent = '失速';
                stallEl.className = 'alert-critical';
            } else {
                stallEl.textContent = '正常';
                stallEl.className = 'status-ok';
            }
        }

        this.charts.updateAerodynamic(data);
    }

    updateOptData(data) {
        const el = id => document.getElementById(id);
        if (el('opt-angle-before')) el('opt-angle-before').textContent = data.current_angle.toFixed(1) + '°';
        if (el('opt-speed-before')) el('opt-speed-before').textContent = data.current_speed?.toFixed(2) + ' m/s' || '--';
        if (el('opt-angle-after')) el('opt-angle-after').textContent = data.optimal_angle.toFixed(1) + '°';
        if (el('opt-speed-after')) el('opt-speed-after').textContent = data.predicted_speed.toFixed(2) + ' m/s';
        if (el('opt-iter')) el('opt-iter').textContent = data.iterations;
        if (el('opt-hulldrag')) el('opt-hulldrag').textContent = (data.hull_drag / 1000).toFixed(1) + ' kN';
        if (el('opt-thrust')) el('opt-thrust').textContent = (data.predicted_thrust / 1000).toFixed(1) + ' kN';
        if (el('stat-optimize-card')) {
            el('stat-optimize-card').style.display = 'block';
        }
        if (el('stat-optimize')) el('stat-optimize').textContent = data.efficiency_gain.toFixed(1);

        this.charts.updateOptimization(data);
    }

    handleAlert(data) {
        this.alerts.unshift(data);
        if (this.alerts.length > this.maxAlerts) {
            this.alerts.pop();
        }
        this.showAlertToast(data);
        this.updateAlertList();
    }

    showAlertToast(data) {
        const toast = document.createElement('div');
        const severityClass = data.severity === 'critical' ? 'alert-critical' :
                             data.severity === 'warning' ? 'alert-warning' : 'status-ok';
        toast.className = `toast ${severityClass}`;
        toast.innerHTML = `
            <div class="toast-title">${this.getAlertTypeLabel(data.alert_type)}</div>
            <div class="toast-message">${data.message}</div>
        `;
        document.body.appendChild(toast);
        requestAnimationFrame(() => toast.classList.add('show'));
        setTimeout(() => {
            toast.classList.remove('show');
            setTimeout(() => toast.remove(), 300);
        }, 5000);
    }

    updateAlertList() {
        const container = document.getElementById('alert-list');
        if (!container) return;

        const counts = { critical: 0, warning: 0, info: 0 };
        this.alerts.forEach(a => counts[a.severity]++);

        const summaryEl = document.getElementById('alert-summary');
        if (summaryEl) {
            summaryEl.innerHTML = `
                <div class="alert-summary-item critical">严重: ${counts.critical}</div>
                <div class="alert-summary-item warning">警告: ${counts.warning}</div>
                <div class="alert-summary-item info">信息: ${counts.info}</div>
            `;
        }

        container.innerHTML = this.alerts.slice(0, 20).map(a => {
            const sevClass = a.severity === 'critical' ? 'alert-critical' :
                            a.severity === 'warning' ? 'alert-warning' : 'status-ok';
            const time = new Date(a.time).toLocaleString();
            return `
                <div class="alert-item ${sevClass}">
                    <div class="alert-header">
                        <span class="alert-type">${this.getAlertTypeLabel(a.alert_type)}</span>
                        <span class="alert-time">${time}</span>
                    </div>
                    <div class="alert-message">${a.message}</div>
                    ${a.current_value !== undefined ? `<div class="alert-details">当前: ${a.current_value.toFixed(2)} / 阈值: ${a.threshold_value?.toFixed(2)}</div>` : ''}
                </div>
            `;
        }).join('');
    }

    updateSailAngle(sailIndex, angleDeg) {
        const sailCards = document.querySelectorAll('.sail-card');
        if (sailCards[sailIndex]) {
            const angleEl = sailCards[sailIndex].querySelector('.sail-angle-value');
            if (angleEl) angleEl.textContent = `${angleDeg.toFixed(1)}°`;
        }
    }

    getCompassDirection(deg) {
        const dirs = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
        const idx = Math.round(((deg % 360) / 45)) % 8;
        return dirs[idx];
    }

    getAlertTypeLabel(type) {
        const labels = {
            'stall': '帆面失速',
            'low_speed': '航速过低',
            'high_wind': '风速过高'
        };
        return labels[type] || type;
    }

    async loadPolarCurve(sailID, windSpeed) {
        try {
            const res = await fetch(`/api/polar-curve?sail_id=${sailID}&wind_speed=${windSpeed}`);
            const data = await res.json();
            this.charts.updatePolarCurve(data);
            this.charts.updateLDCurve(data);
        } catch (e) {
            console.error('Failed to load polar curve:', e);
        }
    }

    async optimizeSail(req) {
        try {
            const res = await fetch('/api/optimize', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(req)
            });
            return await res.json();
        } catch (e) {
            console.error('Optimization failed:', e);
            return null;
        }
    }
}
