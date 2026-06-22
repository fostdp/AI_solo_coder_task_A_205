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

        this.createParticles();
        this.createStreamlines();
    }

    createParticles() {
        const geometry = new THREE.BufferGeometry();
        const positions = new Float32Array(this.particleCount * 3);
        const velocities = new Float32Array(this.particleCount * 3);
        const colors = new Float32Array(this.particleCount * 3);
        const sizes = new Float32Array(this.particleCount);
        this.particleVelocities = velocities;

        for (let i = 0; i < this.particleCount; i++) {
            this.resetParticle(positions, velocities, i, true);
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

    resetParticle(positions, velocities, index, randomY = false) {
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
        const lineCount = 15;
        const pointsPerLine = 80;

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
            line.userData = { lineIndex: l, yOffset: 5 + t * 15, zOffset: (t - 0.5) * 20 };
            this.streamlines.push(line);
            this.scene.add(line);

            this.updateStreamline(line, 0);
        }
    }

    updateStreamline(line, timeOffset) {
        const positions = line.geometry.attributes.position.array;
        const pointsPerLine = positions.length / 3;
        const { yOffset, zOffset } = line.userData;
        const windRad = this.windDirection * Math.PI / 180;

        let x = -40;
        let y = yOffset;
        let z = zOffset;

        const stepSize = 0.8;

        for (let i = 0; i < pointsPerLine; i++) {
            positions[i * 3] = x;
            positions[i * 3 + 1] = y;
            positions[i * 3 + 2] = z;

            const sailDisturbance = this.calculateSailDisturbance(x, y, z, timeOffset);
            const stallTurbulence = this.calculateStallTurbulence(x, y, z);

            const baseVx = Math.sin(windRad) * this.windSpeed;
            const baseVz = Math.cos(windRad) * this.windSpeed;
            const baseVy = Math.sin((x + timeOffset * this.windSpeed) * 0.1) * 0.3;

            const vx = baseVx + sailDisturbance.x + stallTurbulence.x;
            const vy = baseVy + sailDisturbance.y + stallTurbulence.y;
            const vz = baseVz + sailDisturbance.z + stallTurbulence.z;

            const speed = Math.sqrt(vx * vx + vy * vy + vz * vz);
            const norm = speed > 0 ? stepSize / speed : stepSize;

            x += vx * norm;
            y += vy * norm;
            z += vz * norm;

            if (y < 2) y = 2 + Math.random();
            if (y > 30) y = 30 - Math.random();
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
                tx += (Math.random() - 0.5) * intensity * turbulenceScale;
                ty += (Math.random() - 0.3) * intensity * turbulenceScale;
                tz += (Math.random() - 0.5) * intensity * turbulenceScale;

                tx += Math.sin(x * 0.5 + this.time * 3 + idx) * intensity;
                ty += Math.cos(y * 0.4 + this.time * 2.5 + idx) * intensity * 0.7;
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
        const vorticesCount = stalled ? 30 : 15;
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

        this.vortexLines = group;
        this.scene.add(group);
    }

    setWind(speed, direction) {
        this.windSpeed = speed;
        this.windDirection = direction;
    }

    setStall(intensity) {
        this.stallIntensity = Math.max(0, Math.min(1, intensity));

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

                const windRad = this.windDirection * Math.PI / 180;
                const downstreamX = Math.sin(windRad) * 60;
                const downstreamZ = Math.cos(windRad) * 60;

                if (positions[i * 3] > 60 ||
                    positions[i * 3] < -60 ||
                    positions[i * 3 + 1] > 35 ||
                    positions[i * 3 + 1] < 1.5 ||
                    Math.abs(positions[i * 3 + 2]) > 40) {
                    this.resetParticle(positions, this.particleVelocities, i);
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
                this.updateStreamline(line, this.time + idx * 0.5);
            });
        } else {
            this.streamlines.forEach(line => line.visible = false);
        }

        if (this.vortexLines) {
            this.vortexLines.visible = this.showVortices;
        }
    }
}
