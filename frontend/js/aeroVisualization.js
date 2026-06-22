import * as THREE from 'three';

export class AeroVisualization {
    constructor(scene, shipModel) {
        this.scene = scene;
        this.shipModel = shipModel;
        this.particles = null;
        this.particleCount = 200;
        this.streamlines = [];
        this.vortexLines = null;
        this.flowField = null;
        this.windSpeed = 8;
        this.windDirection = 135;
        this.showParticles = true;
        this.showStreamlines = true;
        this.showVortices = false;
        this.time = 0;
        this.stallIntensity = 0;

        this.vorticityField = null;
        this.useVorticitySeeding = true;
        this.vorticityCache = new Map();
        this.sailPositions = [
            { x: 2, y: 13, z: 0, width: 12, height: 16, circulation: 1.0 },
            { x: 9, y: 11.5, z: 0, width: 10, height: 13, circulation: 0.8 },
            { x: -8, y: 10, z: 0, width: 8, height: 11, circulation: 0.6 },
        ];

        this.createVorticityField();
        this.createParticles();
        this.createStreamlines();
    }

    createVorticityField() {
        this.vorticityField = {
            gridMin: new THREE.Vector3(-50, 0, -30),
            gridMax: new THREE.Vector3(60, 30, 30),
            resolution: { x: 30, y: 20, z: 20 },
            cellSize: new THREE.Vector3(),
            magnitude: null,
            maxVorticity: 0,
        };

        const cells = this.vorticityField.resolution.x * this.vorticityField.resolution.y * this.vorticityField.resolution.z;
        this.vorticityField.magnitude = new Float32Array(cells);

        const dx = (this.vorticityField.gridMax.x - this.vorticityField.gridMin.x) / this.vorticityField.resolution.x;
        const dy = (this.vorticityField.gridMax.y - this.vorticityField.gridMin.y) / this.vorticityField.resolution.y;
        const dz = (this.vorticityField.gridMax.z - this.vorticityField.gridMin.z) / this.vorticityField.resolution.z;
        this.vorticityField.cellSize.set(dx, dy, dz);

        this.updateVorticityField();
    }

    updateVorticityField() {
        if (!this.vorticityField) return;

        const { gridMin, resolution, cellSize, magnitude } = this.vorticityField;
        let maxVort = 0;

        for (let iz = 0; iz < resolution.z; iz++) {
            for (let iy = 0; iy < resolution.y; iy++) {
                for (let ix = 0; ix < resolution.x; ix++) {
                    const x = gridMin.x + ix * cellSize.x;
                    const y = gridMin.y + iy * cellSize.y;
                    const z = gridMin.z + iz * cellSize.z;

                    const vort = this.computeVorticityMagnitude(x, y, z);
                    const idx = iz * resolution.y * resolution.x + iy * resolution.x + ix;
                    magnitude[idx] = vort;
                    if (vort > maxVort) maxVort = vort;
                }
            }
        }

        this.vorticityField.maxVorticity = maxVort;
    }

    computeVorticityMagnitude(x, y, z) {
        let totalVort = 0;

        this.sailPositions.forEach((sail) => {
            const sx = sail.x;
            const syTop = sail.y + sail.height / 2;
            const syBot = sail.y - sail.height / 2;
            const gamma = sail.circulation * this.windSpeed * 0.5;

            const tipVortices = [
                { y: syTop, sign: 1 },
                { y: syBot, sign: -1 },
            ];

            tipVortices.forEach((tip) => {
                const dx = x - sx;
                const dy = y - tip.y;
                const dz = z - sail.z;

                const r2d = Math.sqrt(dx * dx + dz * dz);
                const r2 = dx * dx + dy * dy + dz * dz;
                const r = Math.sqrt(r2);

                if (r < 0.5) return;

                const coreRadius = 1.5;
                const sigma = coreRadius;
                const vortMag = (tip.sign * gamma) / (2 * Math.PI * (r2 + sigma * sigma));

                if (dx > -3 && dx < 40) {
                    const wakeFactor = Math.exp(-Math.abs(dy) / 8) * Math.exp(-r2d / 15);
                    totalVort += Math.abs(vortMag) * 5 * wakeFactor;
                }
            });

            const dxS = x - sail.x;
            const dzS = z - sail.z;
            const distX = Math.abs(dxS) / sail.width;
            const distZ = Math.abs(dzS) / (sail.width * 0.6);
            const dist = Math.sqrt(distX * distX + distZ * distZ);

            if (dist < 1.5) {
                const sheetVort = Math.exp(-dist * 3) * gamma * 3;
                totalVort += sheetVort;
            }
        });

        if (this.stallIntensity > 0.01) {
            this.sailPositions.forEach((sail) => {
                const sx = sail.x + sail.width * 0.3;
                const dx = x - sx;
                const dz = z - sail.z;
                const dy = y - sail.y;

                if (dx > -5 && dx < 30) {
                    const shear = this.stallIntensity * Math.exp(-Math.abs(dx) / 15) * Math.exp(-(dy * dy) / 100) * Math.exp(-(dz * dz) / 60);
                    totalVort += shear * 8;
                }
            });
        }

        return totalVort;
    }

    sampleVorticityCDF() {
        const samples = 200;
        const cdf = new Float32Array(samples + 1);
        const positions = [];

        for (let i = 0; i < samples; i++) {
            const t = i / (samples - 1);
            const x = -50 + t * 110;
            const z = -15 + Math.random() * 30;
            const y = 3 + Math.random() * 24;

            const vort = this.computeVorticityMagnitude(x, y, z) + 0.05;
            cdf[i + 1] = cdf[i] + vort;
            positions.push({ x, y, z, weight: vort });
        }

        const total = cdf[samples];
        for (let i = 0; i <= samples; i++) {
            cdf[i] /= total;
        }

        return { cdf, positions, total };
    }

    seedByVorticity() {
        const r = Math.random();
        const { cdf, positions } = this.sampleVorticityCDF();

        let lo = 0;
        let hi = cdf.length - 1;
        while (lo < hi) {
            const mid = (lo + hi) >> 1;
            if (cdf[mid] < r) lo = mid + 1;
            else hi = mid;
        }

        const idx = Math.min(lo, positions.length - 1);
        const basePos = positions[idx];

        return {
            x: basePos.x + (Math.random() - 0.5) * 3,
            y: basePos.y + (Math.random() - 0.5) * 3,
            z: basePos.z + (Math.random() - 0.5) * 3,
            weight: basePos.weight,
        };
    }

    createParticles() {
        const geometry = new THREE.BufferGeometry();
        const positions = new Float32Array(this.particleCount * 3);
        const velocities = new Float32Array(this.particleCount * 3);
        const colors = new Float32Array(this.particleCount * 3);
        const sizes = new Float32Array(this.particleCount);
        const lifetimes = new Float32Array(this.particleCount);

        this.particleVelocities = velocities;
        this.particleLifetimes = lifetimes;

        for (let i = 0; i < this.particleCount; i++) {
            this.resetParticleVorticity(positions, velocities, lifetimes, i);
            const t = i / this.particleCount;
            colors[i * 3] = 0.2 + 0.4 * t;
            colors[i * 3 + 1] = 0.5 + 0.3 * Math.sin(t * Math.PI * 2);
            colors[i * 3 + 2] = 1.0;
            sizes[i] = 0.1 + Math.random() * 0.15;
        }

        geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
        geometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1));

        const material = new THREE.PointsMaterial({
            size: 0.25,
            vertexColors: true,
            transparent: true,
            opacity: 0.85,
            sizeAttenuation: true,
            blending: THREE.AdditiveBlending,
            depthWrite: false,
        });

        this.particles = new THREE.Points(geometry, material);
        this.scene.add(this.particles);
    }

    resetParticleVorticity(positions, velocities, lifetimes, index) {
        const windRad = this.windDirection * Math.PI / 180;

        if (this.useVorticitySeeding && Math.random() < 0.65) {
            const seed = this.seedByVorticity();
            positions[index * 3] = seed.x;
            positions[index * 3 + 1] = seed.y;
            positions[index * 3 + 2] = seed.z;

            const speedJitter = 0.7 + Math.random() * 0.6;
            velocities[index * 3] = Math.sin(windRad) * this.windSpeed * speedJitter;
            velocities[index * 3 + 2] = Math.cos(windRad) * this.windSpeed * speedJitter;
            velocities[index * 3 + 1] = (Math.random() - 0.5) * 0.8;

            lifetimes[index] = 3 + Math.random() * 5;
        } else {
            const spread = 30;
            positions[index * 3] = -50 + Math.random() * 20;
            positions[index * 3 + 1] = 3 + Math.random() * 20;
            positions[index * 3 + 2] = -spread / 2 + Math.random() * spread;

            const speedJitter = 0.8 + Math.random() * 0.4;
            velocities[index * 3] = Math.sin(windRad) * this.windSpeed * speedJitter;
            velocities[index * 3 + 2] = Math.cos(windRad) * this.windSpeed * speedJitter;
            velocities[index * 3 + 1] = (Math.random() - 0.5) * 0.5;

            lifetimes[index] = 8 + Math.random() * 10;
        }
    }

    resetParticle(positions, velocities, index, randomY = false) {
        if (this.particleLifetimes && this.useVorticitySeeding) {
            this.resetParticleVorticity(positions, velocities, this.particleLifetimes, index);
            return;
        }

        const windRad = this.windDirection * Math.PI / 180;

        const spread = 30;
        positions[index * 3] = -50 + Math.random() * 20;
        positions[index * 3 + 1] = randomY ? 3 + Math.random() * 20 : 5 + Math.random() * 15;
        positions[index * 3 + 2] = -spread / 2 + Math.random() * spread;

        const speedJitter = 0.8 + Math.random() * 0.4;
        velocities[index * 3] = Math.sin(windRad) * this.windSpeed * speedJitter;
        velocities[index * 3 + 2] = Math.cos(windRad) * this.windSpeed * speedJitter;
        velocities[index * 3 + 1] = (Math.random() - 0.5) * 0.5;
    }

    setParticleDensity(count) {
        this.particleCount = count;
        if (this.particles) {
            this.scene.remove(this.particles);
        }
        this.createParticles();
    }

    createStreamlines() {
        const lineCount = 18;
        const pointsPerLine = 100;

        for (let l = 0; l < lineCount; l++) {
            const positions = new Float32Array(pointsPerLine * 3);
            const geometry = new THREE.BufferGeometry();
            geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));

            const t = l / lineCount;
            const hue = 0.55 + t * 0.15;
            const material = new THREE.LineBasicMaterial({
                color: new THREE.Color().setHSL(hue, 0.8, 0.6),
                transparent: true,
                opacity: 0.6,
                linewidth: 2,
            });

            const line = new THREE.Line(geometry, material);
            line.userData = {
                lineIndex: l,
                yOffset: 5 + t * 15,
                zOffset: (t - 0.5) * 20,
                useVorticitySeed: Math.random() < 0.7,
            };
            this.streamlines.push(line);
            this.scene.add(line);

            this.updateStreamline(line, 0);
        }
    }

    seedStreamlineStart(lineIndex) {
        if (this.useVorticitySeeding && Math.random() < 0.6) {
            const seed = this.seedByVorticity();
            return { x: seed.x, y: seed.y, z: seed.z };
        } else {
            const windRad = this.windDirection * Math.PI / 180;
            return {
                x: -40 + Math.random() * 10,
                y: 5 + (lineIndex / this.streamlines.length) * 18 + Math.random() * 2,
                z: -15 + (lineIndex / this.streamlines.length) * 30,
            };
        }
    }

    adaptiveStepSize(x, y, z, baseStep) {
        const vort = this.computeVorticityMagnitude(x, y, z);
        const maxVort = this.vorticityField?.maxVorticity || 1;
        const vortNorm = Math.min(vort / maxVort, 1);
        const factor = 1.0 - 0.7 * vortNorm;
        return Math.max(0.15, baseStep * factor);
    }

    updateStreamline(line, timeOffset) {
        const positions = line.geometry.attributes.position.array;
        const pointsPerLine = positions.length / 3;
        const windRad = this.windDirection * Math.PI / 180;
        const { lineIndex, useVorticitySeed, yOffset, zOffset } = line.userData;

        let start;
        if (useVorticitySeed) {
            start = this.seedStreamlineStart(lineIndex);
        } else {
            start = { x: -40, y: yOffset, z: zOffset };
        }

        let x = start.x;
        let y = start.y;
        let z = start.z;

        const baseStep = 0.7;

        for (let i = 0; i < pointsPerLine; i++) {
            positions[i * 3] = x;
            positions[i * 3 + 1] = y;
            positions[i * 3 + 2] = z;

            const step = this.adaptiveStepSize(x, y, z, baseStep);

            const sailDisturbance = this.calculateSailDisturbance(x, y, z, timeOffset);
            const stallTurbulence = this.calculateStallTurbulence(x, y, z);

            const baseVx = Math.sin(windRad) * this.windSpeed;
            const baseVz = Math.cos(windRad) * this.windSpeed;
            const baseVy = Math.sin((x + timeOffset * this.windSpeed) * 0.1) * 0.3;

            const vx = baseVx + sailDisturbance.x + stallTurbulence.x;
            const vy = baseVy + sailDisturbance.y + stallTurbulence.y;
            const vz = baseVz + sailDisturbance.z + stallTurbulence.z;

            const speed = Math.sqrt(vx * vx + vy * vy + vz * vz);
            const norm = speed > 0 ? step / speed : step;

            x += vx * norm;
            y += vy * norm;
            z += vz * norm;

            if (y < 2) y = 2 + Math.random() * 0.5;
            if (y > 30) y = 30 - Math.random() * 0.5;
            if (Math.abs(z) > 25) break;
            if (x > 55) break;
        }

        line.geometry.attributes.position.needsUpdate = true;
    }

    calculateSailDisturbance(x, y, z, timeOffset) {
        let dx = 0, dy = 0, dz = 0;
        const sailPositions = [
            { x: 2, y: 13, z: 0, width: 12, height: 16 },
            { x: 9, y: 11.5, z: 0, width: 10, height: 13 },
            { x: -8, y: 10, z: 0, width: 8, height: 11 },
        ];

        sailPositions.forEach((sail, idx) => {
            const dxS = x - sail.x;
            const dyS = y - sail.y;
            const dzS = z - sail.z;

            const distX = Math.abs(dxS) / sail.width;
            const distY = Math.abs(dyS) / sail.height;
            const distZ = Math.abs(dzS) / (sail.width * 0.8);

            const dist = Math.sqrt(distX * distX + distY * distY + distZ * distZ);

            if (dist < 2.5) {
                const influence = Math.exp(-dist * 1.2) * 3;
                const upwash = Math.exp(-distZ * 3) * influence;

                dx += -dxS * 0.02 * influence;
                dy += (0.5 + Math.sin(timeOffset * 2 + idx) * 0.3) * upwash * (dxS < 0 ? 1 : -0.5);
                dz += -dzS * 0.05 * influence;
            }
        });

        return { x: dx, y: dy, z: dz };
    }

    calculateStallTurbulence(x, y, z) {
        if (this.stallIntensity <= 0.01) return { x: 0, y: 0, z: 0 };

        const sailPositions = [
            { x: 2, y: 13, z: 0 },
            { x: 9, y: 11.5, z: 0 },
            { x: -8, y: 10, z: 0 },
        ];

        let tx = 0, ty = 0, tz = 0;

        sailPositions.forEach((sail, idx) => {
            const dx = x - sail.x;
            const dy = y - sail.y;
            const dz = z - sail.z;
            const dist = Math.sqrt(dx * dx + dy * dy + dz * dz);

            if (dx > -5 && dx < 40 && dist < 25) {
                const intensity = this.stallIntensity * Math.exp(-dist * 0.1) * 2;
                const turbulenceScale = 2 + dist * 0.1;

                const phase = x * 0.5 + this.time * 3 + idx;
                tx += Math.sin(phase) * intensity * turbulenceScale;
                ty += Math.cos(y * 0.4 + this.time * 2.5 + idx) * intensity * 0.7;
                tz += Math.sin(z * 0.6 + this.time * 2 + idx) * intensity * 0.8;
            }
        });

        return { x: tx, y: ty, z: tz };
    }

    createVortexLines(aoa, cl, stalled) {
        if (this.vortexLines) {
            this.scene.remove(this.vortexLines);
        }

        if (!this.showVortices) return;

        const group = new THREE.Group();
        const tipPositions = [
            { x: 2, y: 20, z1: 6, z2: -6 },
            { x: 9, y: 17, z1: 5, z2: -5 },
            { x: -8, y: 14, z1: 4, z2: -4 },
        ];

        const lineMaterial = new THREE.LineBasicMaterial({
            color: stalled ? 0xef4444 : 0x8b5cf6,
            transparent: true,
            opacity: 0.7,
        });

        tipPositions.forEach((tip) => {
            [[tip.x, tip.y, tip.z1], [tip.x, tip.y, tip.z2]].forEach(([sx, sy, sz]) => {
                const points = [];
                const helices = 3;
                for (let t = 0; t <= 1; t += 0.02) {
                    const windRad = this.windDirection * Math.PI / 180;
                    const helixRadius = 0.5 + t * 3;
                    const angle = t * Math.PI * 2 * helices;
                    points.push(new THREE.Vector3(
                        sx + Math.sin(windRad) * t * 30 + Math.cos(angle) * helixRadius,
                        sy + Math.sin(angle) * helixRadius * 0.5,
                        sz + Math.cos(windRad) * t * 30 + Math.sin(angle) * helixRadius,
                    ));
                }
                const g = new THREE.BufferGeometry().setFromPoints(points);
                group.add(new THREE.Line(g, lineMaterial));
            });
        });

        if (this.useVorticitySeeding && stalled) {
            const ringCount = 12;
            const ringMaterial = new THREE.LineBasicMaterial({
                color: 0xf59e0b,
                transparent: true,
                opacity: 0.5,
            });
            for (let r = 0; r < ringCount; r++) {
                const t = r / (ringCount - 1);
                const x0 = 2 + t * 25;
                const radius = 1 + t * 4;
                const ringPoints = [];
                for (let a = 0; a < Math.PI * 2; a += 0.3) {
                    ringPoints.push(new THREE.Vector3(
                        x0,
                        13 + Math.sin(a) * radius,
                        Math.cos(a) * radius,
                    ));
                }
                const rg = new THREE.BufferGeometry().setFromPoints(ringPoints);
                group.add(new THREE.Line(rg, ringMaterial));
            }
        }

        this.vortexLines = group;
        this.scene.add(group);
    }

    setWind(speed, direction) {
        const oldSpeed = this.windSpeed;
        const oldDir = this.windDirection;
        this.windSpeed = speed;
        this.windDirection = direction;

        if (Math.abs(speed - oldSpeed) > 2 || Math.abs(direction - oldDir) > 10) {
            this.updateVorticityField();
        }
    }

    setStall(intensity) {
        const old = this.stallIntensity;
        this.stallIntensity = Math.max(0, Math.min(1, intensity));

        if (Math.abs(this.stallIntensity - old) > 0.1) {
            this.updateVorticityField();
        }

        if (this.particles) {
            const colors = this.particles.geometry.attributes.color;
            for (let i = 0; i < this.particleCount; i++) {
                const r = colors.array[i * 3];
                colors.array[i * 3] = r + this.stallIntensity * 0.5;
                colors.array[i * 3 + 2] = Math.max(0.3, 1 - this.stallIntensity * 0.6);
            }
            colors.needsUpdate = true;
        }
    }

    update(deltaTime) {
        this.time += deltaTime;

        if (this.showParticles && this.particles) {
            const positions = this.particles.geometry.attributes.position.array;

            for (let i = 0; i < this.particleCount; i++) {
                let vx = this.particleVelocities[i * 3];
                let vy = this.particleVelocities[i * 3 + 1];
                let vz = this.particleVelocities[i * 3 + 2];

                const x = positions[i * 3];
                const y = positions[i * 3 + 1];
                const z = positions[i * 3 + 2];

                const disturb = this.calculateSailDisturbance(x, y, z, this.time);
                const stall = this.calculateStallTurbulence(x, y, z);

                vx += (disturb.x + stall.x) * deltaTime * 2;
                vy += (disturb.y + stall.y) * deltaTime * 2;
                vz += (disturb.z + stall.z) * deltaTime * 2;

                vy += Math.sin(x * 0.1 + this.time * 2) * deltaTime;

                positions[i * 3] += vx * deltaTime;
                positions[i * 3 + 1] += vy * deltaTime;
                positions[i * 3 + 2] += vz * deltaTime;

                this.particleLifetimes[i] -= deltaTime;

                const windRad = this.windDirection * Math.PI / 180;

                const outOfBounds =
                    positions[i * 3] > 60 ||
                    positions[i * 3] < -60 ||
                    positions[i * 3 + 1] > 35 ||
                    positions[i * 3 + 1] < 1.5 ||
                    Math.abs(positions[i * 3 + 2]) > 40;

                if (outOfBounds || this.particleLifetimes[i] <= 0) {
                    this.resetParticleVorticity(
                        positions,
                        this.particleVelocities,
                        this.particleLifetimes,
                        i,
                    );
                }
            }

            this.particles.geometry.attributes.position.needsUpdate = true;
        }

        if (this.particles) {
            this.particles.visible = this.showParticles;
        }

        if (this.showStreamlines) {
            this.streamlines.forEach((line, idx) => {
                line.visible = true;
                if (idx % 3 === 0 || this.time % 0.5 < 0.016) {
                    this.updateStreamline(line, this.time + idx * 0.5);
                }
            });
        } else {
            this.streamlines.forEach((line) => { line.visible = false; });
        }

        if (this.vortexLines) {
            this.vortexLines.visible = this.showVortices;
        }
    }
}
