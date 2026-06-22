export class Charts {
    constructor() {
        this.charts = {};
        this.histories = {
            cl: [],
            cd: [],
            aoa: [],
            ld: [],
            opt: [],
            speedInitial: [],
            speedOptimized: [],
        };
        this.maxHistory = 50;
        this.polarData = null;

        this.initCharts();
    }

    initCharts() {
        ['coefficients', 'optimization', 'polar', 'ld'].forEach(id => {
            const canvas = document.getElementById('chart-' + id);
            if (canvas) {
                this.charts[id] = canvas;
                this.setupCanvas(canvas);
            }
        });
    }

    setupCanvas(canvas) {
        const dpr = window.devicePixelRatio || 1;
        const rect = canvas.getBoundingClientRect();
        canvas.width = rect.width * dpr;
        canvas.height = rect.height * dpr;
        const ctx = canvas.getContext('2d');
        ctx.scale(dpr, dpr);
    }

    updateAerodynamic(aoa, cl, cd, isStalled) {
        this.histories.aoa.push({ t: Date.now(), v: aoa });
        this.histories.cl.push({ t: Date.now(), v: cl });
        this.histories.cd.push({ t: Date.now(), v: cd });
        this.histories.ld.push({ t: Date.now(), v: cd > 0.001 ? cl / cd : 0 });

        if (this.histories.aoa.length > this.maxHistory) {
            this.histories.aoa.shift();
            this.histories.cl.shift();
            this.histories.cd.shift();
            this.histories.ld.shift();
        }

        this.drawCoefficients(isStalled);
    }

    drawCoefficients(isStalled) {
        const canvas = this.charts.coefficients;
        if (!canvas) return;
        this.setupCanvas(canvas);
        const ctx = canvas.getContext('2d');
        const rect = canvas.getBoundingClientRect();
        const W = rect.width, H = rect.height;

        ctx.fillStyle = '#0f172a';
        ctx.fillRect(0, 0, W, H);

        const pad = { l: 45, r: 15, t: 25, b: 30 };
        const cw = W - pad.l - pad.r;
        const ch = H - pad.t - pad.b;

        const clMax = 2.5, clMin = -0.5;
        const cdMax = 1.0, cdMin = 0;

        ctx.strokeStyle = '#334155';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 5; i++) {
            const y = pad.t + ch - (ch * i / 5);
            ctx.beginPath();
            ctx.moveTo(pad.l, y);
            ctx.lineTo(pad.l + cw, y);
            ctx.stroke();
        }

        ctx.fillStyle = '#64748b';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        for (let i = 0; i <= 5; i++) {
            const val = clMin + (clMax - clMin) * i / 5;
            const y = pad.t + ch - (ch * i / 5);
            ctx.fillText(val.toFixed(1), pad.l - 5, y + 3);
        }

        if (this.histories.cl.length >= 2) {
            const n = this.histories.cl.length;

            ctx.strokeStyle = '#10b981';
            ctx.lineWidth = 2;
            ctx.beginPath();
            this.histories.cl.forEach((pt, i) => {
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - ((pt.v - clMin) / (clMax - clMin)) * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            ctx.strokeStyle = '#ef4444';
            ctx.lineWidth = 2;
            ctx.beginPath();
            this.histories.cd.forEach((pt, i) => {
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - ((pt.v - cdMin) / (cdMax - cdMin)) * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            ctx.strokeStyle = '#8b5cf6';
            ctx.lineWidth = 1.5;
            ctx.setLineDash([4, 4]);
            ctx.beginPath();
            this.histories.ld.forEach((pt, i) => {
                const norm = Math.min(pt.v / 20, 1);
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - norm * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();
            ctx.setLineDash([]);
        }

        const legendX = pad.l + 10;
        const legendY = pad.t + 12;
        this.drawLegend(ctx, legendX, legendY, [
            { color: '#10b981', label: 'Cl (升力系数)' },
            { color: '#ef4444', label: 'Cd (阻力系数)' },
            { color: '#8b5cf6', label: 'L/D (升阻比/20)' },
        ]);

        if (isStalled) {
            ctx.fillStyle = 'rgba(239, 68, 68, 0.15)';
            ctx.fillRect(pad.l, pad.t, cw, ch);
            ctx.fillStyle = '#ef4444';
            ctx.font = 'bold 12px sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText('⚠ 失速状态', W / 2, pad.t + 18);
        }

        ctx.fillStyle = '#94a3b8';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('时间序列 →', W / 2, H - 8);
    }

    drawLegend(ctx, x, y, items) {
        ctx.font = '10px sans-serif';
        items.forEach((item, i) => {
            const iy = y + i * 14;
            ctx.fillStyle = item.color;
            ctx.fillRect(x, iy - 8, 12, 3);
            ctx.fillStyle = '#94a3b8';
            ctx.textAlign = 'left';
            ctx.fillText(item.label, x + 18, iy - 4);
        });
    }

    updateOptimization(result) {
        this.histories.opt.push({
            t: Date.now(),
            iter: result.iterations || 0,
            speedIncrease: result.speed_increase || 0,
            initial: result.initial_ship_speed || 0,
            optimized: result.optimized_ship_speed || 0,
        });
        this.histories.speedInitial.push({ t: Date.now(), v: result.initial_ship_speed || 0 });
        this.histories.speedOptimized.push({ t: Date.now(), v: result.optimized_ship_speed || 0 });

        if (this.histories.opt.length > this.maxHistory) {
            this.histories.opt.shift();
            this.histories.speedInitial.shift();
            this.histories.speedOptimized.shift();
        }

        this.drawOptimization();
    }

    drawOptimization() {
        const canvas = this.charts.optimization;
        if (!canvas) return;
        this.setupCanvas(canvas);
        const ctx = canvas.getContext('2d');
        const rect = canvas.getBoundingClientRect();
        const W = rect.width, H = rect.height;

        ctx.fillStyle = '#0f172a';
        ctx.fillRect(0, 0, W, H);

        const pad = { l: 45, r: 45, t: 25, b: 30 };
        const cw = W - pad.l - pad.r;
        const ch = H - pad.t - pad.b;

        let maxSpeed = 8;
        if (this.histories.speedOptimized.length) {
            maxSpeed = Math.max(maxSpeed, ...this.histories.speedOptimized.map(d => d.v)) * 1.2;
        }

        ctx.strokeStyle = '#334155';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 5; i++) {
            const y = pad.t + ch - (ch * i / 5);
            ctx.beginPath();
            ctx.moveTo(pad.l, y);
            ctx.lineTo(pad.l + cw, y);
            ctx.stroke();
        }

        ctx.fillStyle = '#64748b';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        for (let i = 0; i <= 5; i++) {
            const val = (maxSpeed * i / 5);
            const y = pad.t + ch - (ch * i / 5);
            ctx.fillText(val.toFixed(1) + 'm/s', pad.l - 5, y + 3);
        }

        if (this.histories.speedInitial.length >= 2) {
            const n = this.histories.speedInitial.length;

            ctx.fillStyle = 'rgba(16, 185, 129, 0.15)';
            ctx.beginPath();
            this.histories.speedInitial.forEach((pt, i) => {
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - (pt.v / maxSpeed) * ch;
                if (i === 0) {
                    ctx.moveTo(x, pad.t + ch);
                    ctx.lineTo(x, y);
                } else {
                    ctx.lineTo(x, y);
                }
            });
            for (let i = n - 1; i >= 0; i--) {
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - (this.histories.speedOptimized[i].v / maxSpeed) * ch;
                ctx.lineTo(x, y);
            }
            ctx.closePath();
            ctx.fill();

            ctx.strokeStyle = '#f59e0b';
            ctx.lineWidth = 2;
            ctx.beginPath();
            this.histories.speedInitial.forEach((pt, i) => {
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - (pt.v / maxSpeed) * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            ctx.strokeStyle = '#10b981';
            ctx.lineWidth = 2.5;
            ctx.beginPath();
            this.histories.speedOptimized.forEach((pt, i) => {
                const x = pad.l + (i / (n - 1)) * cw;
                const y = pad.t + ch - (pt.v / maxSpeed) * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();
        }

        const legendX = pad.l + 10;
        const legendY = pad.t + 12;
        this.drawLegend(ctx, legendX, legendY, [
            { color: '#f59e0b', label: '当前航速' },
            { color: '#10b981', label: '优化后航速' },
        ]);

        ctx.fillStyle = '#94a3b8';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('优化迭代 →', W / 2, H - 8);
    }

    updatePolarCurve(polars) {
        this.polarData = polars;
        this.drawPolar();
        this.drawLD();
    }

    drawPolar() {
        const canvas = this.charts.polar;
        if (!canvas || !this.polarData) return;
        this.setupCanvas(canvas);
        const ctx = canvas.getContext('2d');
        const rect = canvas.getBoundingClientRect();
        const W = rect.width, H = rect.height;

        ctx.fillStyle = '#0f172a';
        ctx.fillRect(0, 0, W, H);

        const pad = { l: 50, r: 20, t: 25, b: 40 };
        const cw = W - pad.l - pad.r;
        const ch = H - pad.t - pad.b;

        const cdMax = 0.5;
        const clMax = 2.5;
        const clMin = -0.5;

        ctx.strokeStyle = '#334155';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 5; i++) {
            const y = pad.t + ch - (ch * i / 5);
            ctx.beginPath();
            ctx.moveTo(pad.l, y);
            ctx.lineTo(pad.l + cw, y);
            ctx.stroke();

            const x = pad.l + (cw * i / 5);
            ctx.beginPath();
            ctx.moveTo(x, pad.t);
            ctx.lineTo(x, pad.t + ch);
            ctx.stroke();
        }

        ctx.fillStyle = '#64748b';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        for (let i = 0; i <= 5; i++) {
            const val = clMin + (clMax - clMin) * i / 5;
            const y = pad.t + ch - (ch * i / 5);
            ctx.fillText(val.toFixed(1), pad.l - 5, y + 3);
        }
        ctx.textAlign = 'center';
        for (let i = 0; i <= 5; i++) {
            const val = cdMax * i / 5;
            const x = pad.l + (cw * i / 5);
            ctx.fillText(val.toFixed(2), x, H - 12);
        }

        if (this.polarData.length >= 2) {
            ctx.strokeStyle = '#06b6d4';
            ctx.lineWidth = 2;
            ctx.beginPath();

            this.polarData.forEach((p, i) => {
                const x = pad.l + (Math.min(p.drag_coefficient, cdMax) / cdMax) * cw;
                const y = pad.t + ch - ((p.lift_coefficient - clMin) / (clMax - clMin)) * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            ctx.fillStyle = '#06b6d4';
            this.polarData.forEach((p) => {
                const x = pad.l + (Math.min(p.drag_coefficient, cdMax) / cdMax) * cw;
                const y = pad.t + ch - ((p.lift_coefficient - clMin) / (clMax - clMin)) * ch;
                if (Math.abs(p.angle_of_attack % 5) < 0.5) {
                    ctx.beginPath();
                    ctx.arc(x, y, 3, 0, Math.PI * 2);
                    ctx.fill();

                    if (Math.abs(p.angle_of_attack % 10) < 0.5) {
                        ctx.fillStyle = '#94a3b8';
                        ctx.font = '9px sans-serif';
                        ctx.textAlign = 'left';
                        ctx.fillText(p.angle_of_attack + '°', x + 5, y - 3);
                        ctx.fillStyle = '#06b6d4';
                    }
                }
            });

            let bestLD = 0;
            let bestP = null;
            this.polarData.forEach(p => {
                if (p.lift_to_drag_ratio > bestLD) {
                    bestLD = p.lift_to_drag_ratio;
                    bestP = p;
                }
            });

            if (bestP) {
                const x = pad.l + (Math.min(bestP.drag_coefficient, cdMax) / cdMax) * cw;
                const y = pad.t + ch - ((bestP.lift_coefficient - clMin) / (clMax - clMin)) * ch;
                ctx.strokeStyle = '#10b981';
                ctx.lineWidth = 1;
                ctx.setLineDash([4, 2]);
                ctx.beginPath();
                ctx.moveTo(pad.l, y);
                ctx.lineTo(x, y);
                ctx.lineTo(x, pad.t + ch);
                ctx.stroke();
                ctx.setLineDash([]);

                ctx.fillStyle = '#10b981';
                ctx.beginPath();
                ctx.arc(x, y, 6, 0, Math.PI * 2);
                ctx.fill();
                ctx.fillStyle = 'white';
                ctx.font = 'bold 9px sans-serif';
                ctx.textAlign = 'center';
                ctx.fillText('★', x, y + 3);
            }
        }

        ctx.fillStyle = '#94a3b8';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('Cd (阻力系数)', W / 2, H - 5);
        ctx.save();
        ctx.translate(12, H / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.fillText('Cl (升力系数)', 0, 0);
        ctx.restore();

        ctx.textAlign = 'right';
        ctx.font = '10px sans-serif';
        ctx.fillStyle = '#94a3b8';
        ctx.fillText(`最佳 L/D: ${bestLD ? bestLD.toFixed(1) : '--'}`, W - pad.r, H - 25);
    }

    drawLD() {
        const canvas = this.charts.ld;
        if (!canvas || !this.polarData) return;
        this.setupCanvas(canvas);
        const ctx = canvas.getContext('2d');
        const rect = canvas.getBoundingClientRect();
        const W = rect.width, H = rect.height;

        ctx.fillStyle = '#0f172a';
        ctx.fillRect(0, 0, W, H);

        const pad = { l: 50, r: 20, t: 25, b: 40 };
        const cw = W - pad.l - pad.r;
        const ch = H - pad.t - pad.b;

        const aoaMin = -20;
        const aoaMax = 30;
        let ldMax = 20;
        if (this.polarData.length) {
            ldMax = Math.max(ldMax, ...this.polarData.map(p => p.lift_to_drag_ratio)) * 1.1;
        }

        ctx.strokeStyle = '#334155';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 5; i++) {
            const y = pad.t + ch - (ch * i / 5);
            ctx.beginPath();
            ctx.moveTo(pad.l, y);
            ctx.lineTo(pad.l + cw, y);
            ctx.stroke();
        }
        for (let aoa = -20; aoa <= 30; aoa += 10) {
            const x = pad.l + ((aoa - aoaMin) / (aoaMax - aoaMin)) * cw;
            ctx.beginPath();
            ctx.moveTo(x, pad.t);
            ctx.lineTo(x, pad.t + ch);
            ctx.strokeStyle = (aoa === 0) ? '#475569' : '#334155';
            ctx.stroke();
        }

        ctx.fillStyle = '#64748b';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        for (let i = 0; i <= 5; i++) {
            const val = ldMax * i / 5;
            const y = pad.t + ch - (ch * i / 5);
            ctx.fillText(val.toFixed(0), pad.l - 5, y + 3);
        }
        ctx.textAlign = 'center';
        for (let aoa = -20; aoa <= 30; aoa += 10) {
            const x = pad.l + ((aoa - aoaMin) / (aoaMax - aoaMin)) * cw;
            ctx.fillText(aoa + '°', x, H - 12);
        }

        if (this.polarData.length >= 2) {
            const stallAoa = 18;
            const gradient = ctx.createLinearGradient(pad.l, 0, pad.l + cw, 0);
            gradient.addColorStop(0, '#3b82f6');
            gradient.addColorStop(0.5, '#10b981');
            gradient.addColorStop(1, '#ef4444');

            ctx.lineWidth = 2.5;

            ctx.beginPath();
            ctx.strokeStyle = '#10b981';
            let started = false;
            this.polarData.forEach((p, i) => {
                if (Math.abs(p.angle_of_attack) > stallAoa) return;
                const x = pad.l + ((p.angle_of_attack - aoaMin) / (aoaMax - aoaMin)) * cw;
                const y = pad.t + ch - (p.lift_to_drag_ratio / ldMax) * ch;
                if (!started) { ctx.moveTo(x, y); started = true; }
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            ctx.beginPath();
            ctx.strokeStyle = '#ef4444';
            started = false;
            this.polarData.forEach((p) => {
                if (Math.abs(p.angle_of_attack) <= stallAoa) return;
                const x = pad.l + ((p.angle_of_attack - aoaMin) / (aoaMax - aoaMin)) * cw;
                const y = pad.t + ch - (p.lift_to_drag_ratio / ldMax) * ch;
                if (!started) { ctx.moveTo(x, y); started = true; }
                else ctx.lineTo(x, y);
            });
            ctx.setLineDash([5, 3]);
            ctx.stroke();
            ctx.setLineDash([]);

            ctx.fillStyle = 'rgba(239, 68, 68, 0.08)';
            const xLeft = pad.l + ((-stallAoa - aoaMin) / (aoaMax - aoaMin)) * cw;
            const xRight = pad.l + ((stallAoa - aoaMin) / (aoaMax - aoaMin)) * cw;
            ctx.fillRect(pad.l, pad.t, xLeft - pad.l, ch);
            ctx.fillRect(xRight, pad.t, pad.l + cw - xRight, ch);

            let bestLD = 0;
            let bestX = 0;
            let bestY = 0;
            this.polarData.forEach(p => {
                if (p.lift_to_drag_ratio > bestLD && Math.abs(p.angle_of_attack) <= stallAoa) {
                    bestLD = p.lift_to_drag_ratio;
                    bestX = pad.l + ((p.angle_of_attack - aoaMin) / (aoaMax - aoaMin)) * cw;
                    bestY = pad.t + ch - (p.lift_to_drag_ratio / ldMax) * ch;
                }
            });

            if (bestLD > 0) {
                ctx.fillStyle = '#fbbf24';
                ctx.beginPath();
                ctx.moveTo(bestX, bestY - 10);
                ctx.lineTo(bestX + 7, bestY);
                ctx.lineTo(bestX - 7, bestY);
                ctx.closePath();
                ctx.fill();
            }
        }

        ctx.fillStyle = '#94a3b8';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('攻角 AoA (°)', W / 2, H - 5);
        ctx.save();
        ctx.translate(12, H / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.fillText('升阻比 L/D', 0, 0);
        ctx.restore();
    }
}
