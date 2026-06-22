import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

export class ShipModel {
    constructor(scene) {
        this.scene = scene;
        this.shipGroup = new THREE.Group();
        this.sails = [];
        this.masts = [];
        this.rigging = [];
        this.sea = null;
        this.sailAngles = [0, 0, 0];
        this.sailCambers = [0.13, 0.13, 0.13];
        this.sailPositions = [
            { x: -8, y: 8, z: 0, height: 15, width: 10 },
            { x: 0, y: 10, z: 0, height: 18, width: 12 },
            { x: 10, y: 7, z: 0, height: 13, width: 8 }
        ];
        this.mastPositions = [-8, 0, 10];
        this.buildHull();
        this.buildDeck();
        this.buildCabin();
        this.buildMasts();
        this.buildSails();
        this.buildRigging();
        this.buildSea();
        scene.add(this.shipGroup);
    }

    buildHull() {
        const hullShape = new THREE.Shape();
        hullShape.moveTo(-18, -2);
        hullShape.quadraticCurveTo(-20, -3, -19, -5);
        hullShape.quadraticCurveTo(-17, -7, -10, -7);
        hullShape.quadraticCurveTo(0, -7.5, 10, -7);
        hullShape.quadraticCurveTo(17, -7, 19, -5);
        hullShape.quadraticCurveTo(20, -3, 18, -2);
        hullShape.lineTo(18, 2);
        hullShape.quadraticCurveTo(17, 3, 12, 3.5);
        hullShape.quadraticCurveTo(0, 4, -12, 3.5);
        hullShape.quadraticCurveTo(-17, 3, -18, 2);
        hullShape.lineTo(-18, -2);

        const extrudeSettings = {
            depth: 9.8,
            bevelEnabled: true,
            bevelThickness: 0.1,
            bevelSize: 0.1,
            bevelSegments: 1
        };

        const hullGeometry = new THREE.ExtrudeGeometry(hullShape, extrudeSettings);
        hullGeometry.rotateY(-Math.PI / 2);
        hullGeometry.translate(0, 0, 0);

        const hullMaterial = new THREE.MeshStandardMaterial({
            color: 0x8B4513,
            roughness: 0.8,
            metalness: 0.1
        });

        const hull = new THREE.Mesh(hullGeometry, hullMaterial);
        hull.castShadow = true;
        hull.receiveShadow = true;
        this.shipGroup.add(hull);
    }

    buildDeck() {
        const deckGeometry = new THREE.BoxGeometry(34, 0.3, 9);
        const deckMaterial = new THREE.MeshStandardMaterial({
            color: 0xD2691E,
            roughness: 0.9,
            metalness: 0.05
        });
        const deck = new THREE.Mesh(deckGeometry, deckMaterial);
        deck.position.set(0, 1.8, 0);
        deck.castShadow = true;
        deck.receiveShadow = true;
        this.shipGroup.add(deck);
    }

    buildCabin() {
        const cabinGeometry = new THREE.BoxGeometry(8, 2.5, 7);
        const cabinMaterial = new THREE.MeshStandardMaterial({
            color: 0xA0522D,
            roughness: 0.7,
            metalness: 0.1
        });
        const cabin = new THREE.Mesh(cabinGeometry, cabinMaterial);
        cabin.position.set(3, 3.2, 0);
        cabin.castShadow = true;
        cabin.receiveShadow = true;
        this.shipGroup.add(cabin);

        const roofGeometry = new THREE.ConeGeometry(5.5, 1.5, 4);
        const roofMaterial = new THREE.MeshStandardMaterial({
            color: 0x8B0000,
            roughness: 0.8,
            metalness: 0.05
        });
        const roof = new THREE.Mesh(roofGeometry, roofMaterial);
        roof.position.set(3, 5.2, 0);
        roof.rotation.y = Math.PI / 4;
        roof.castShadow = true;
        this.shipGroup.add(roof);
    }

    buildMasts() {
        this.mastPositions.forEach((x, i) => {
            const heights = [14, 17, 12];
            const mastGeometry = new THREE.CylinderGeometry(0.12, 0.25, heights[i], 8);
            const mastMaterial = new THREE.MeshStandardMaterial({
                color: 0x654321,
                roughness: 0.9,
                metalness: 0.05
            });
            const mast = new THREE.Mesh(mastGeometry, mastMaterial);
            mast.position.set(x, heights[i] / 2 + 2, 0);
            mast.castShadow = true;
            this.shipGroup.add(mast);
            this.masts.push(mast);

            const yardGeometry = new THREE.CylinderGeometry(0.08, 0.08, heights[i] * 0.55, 8);
            const yardMaterial = new THREE.MeshStandardMaterial({
                color: 0x654321,
                roughness: 0.9,
                metalness: 0.05
            });
            const yard = new THREE.Mesh(yardGeometry, yardMaterial);
            yard.position.set(x, heights[i] * 0.6 + 2, 0);
            yard.rotation.z = Math.PI / 2;
            yard.castShadow = true;
            this.shipGroup.add(yard);
        });
    }

    buildSails() {
        const sailMaterial = new THREE.MeshStandardMaterial({
            color: 0xE8DCC0,
            side: THREE.DoubleSide,
            roughness: 0.95,
            metalness: 0.0,
            transparent: true,
            opacity: 0.92
        });

        this.sailPositions.forEach((pos, i) => {
            const geometry = new THREE.PlaneGeometry(pos.width, pos.height, 20, 25);
            const sail = new THREE.Mesh(geometry, sailMaterial.clone());
            sail.position.set(pos.x, pos.y, pos.z);
            sail.castShadow = true;
            sail.receiveShadow = true;
            this.shipGroup.add(sail);
            this.sails.push(sail);
        });
    }

    buildRigging() {
        const riggingMaterial = new THREE.LineBasicMaterial({ color: 0x4A3728 });
        this.mastPositions.forEach((x, i) => {
            const points1 = [
                new THREE.Vector3(x, 15, 0),
                new THREE.Vector3(x - 8, 1.5, -4.5)
            ];
            const points2 = [
                new THREE.Vector3(x, 15, 0),
                new THREE.Vector3(x + 8, 1.5, -4.5)
            ];
            const points3 = [
                new THREE.Vector3(x, 15, 0),
                new THREE.Vector3(x - 8, 1.5, 4.5)
            ];
            const points4 = [
                new THREE.Vector3(x, 15, 0),
                new THREE.Vector3(x + 8, 1.5, 4.5)
            ];
            [points1, points2, points3, points4].forEach(points => {
                const geometry = new THREE.BufferGeometry().setFromPoints(points);
                const line = new THREE.Line(geometry, riggingMaterial);
                this.shipGroup.add(line);
                this.rigging.push(line);
            });
        });
    }

    buildSea() {
        const seaGeometry = new THREE.PlaneGeometry(200, 200, 50, 50);
        const seaMaterial = new THREE.MeshStandardMaterial({
            color: 0x1a4c6e,
            roughness: 0.3,
            metalness: 0.4,
            transparent: true,
            opacity: 0.85
        });
        this.sea = new THREE.Mesh(seaGeometry, seaMaterial);
        this.sea.rotation.x = -Math.PI / 2;
        this.sea.position.y = -8;
        this.sea.receiveShadow = true;
        this.scene.add(this.sea);
    }

    updateSailShape(sailIndex, angleDeg, camber) {
        if (sailIndex >= this.sails.length) return;
        const sail = this.sails[sailIndex];
        const sailPos = this.sailPositions[sailIndex];
        const height = sailPos.height;
        const width = sailPos.width;
        const theta = angleDeg * Math.PI / 180;
        this.sailAngles[sailIndex] = angleDeg;
        this.sailCambers[sailIndex] = camber;

        const positions = sail.geometry.attributes.position;
        for (let i = 0; i < positions.count; i++) {
            const u = (i % 21) / 20;
            const v = Math.floor(i / 21) / 25;
            const xOriginal = (u - 0.5) * width;
            const yOriginal = (v - 0.5) * height;
            const camberProfile = 4 * camber * (u - u * u);
            const tensionFactor = Math.sin(v * Math.PI);
            const zDisplacement = camberProfile * height * 0.3 * tensionFactor;
            const xRotated = xOriginal * Math.cos(theta) + zDisplacement * Math.sin(theta);
            const zRotated = -xOriginal * Math.sin(theta) + zDisplacement * Math.cos(theta);
            positions.setXYZ(i, xRotated, yOriginal, zRotated);
        }
        positions.needsUpdate = true;
        sail.geometry.computeVertexNormals();
    }

    update(deltaTime, windSpeed, windDirectionDeg) {
        if (this.sea) {
            const positions = this.sea.geometry.attributes.position;
            const time = Date.now() * 0.001;
            for (let i = 0; i < positions.count; i++) {
                const x = positions.getX(i);
                const z = positions.getZ(i);
                const wave1 = Math.sin(x * 0.1 + time * 0.5) * 0.3;
                const wave2 = Math.sin(z * 0.15 + time * 0.7) * 0.2;
                const wave3 = Math.sin((x + z) * 0.08 + time * 0.3) * 0.15;
                positions.setY(i, wave1 + wave2 + wave3);
            }
            positions.needsUpdate = true;
            this.sea.geometry.computeVertexNormals();
        }

        const windRad = windDirectionDeg * Math.PI / 180;
        const rollAngle = Math.sin(windRad) * windSpeed * 0.003;
        const pitchAngle = Math.cos(windRad) * windSpeed * 0.002;
        this.shipGroup.rotation.z = rollAngle;
        this.shipGroup.rotation.x = pitchAngle;
    }

    getShipGroup() {
        return this.shipGroup;
    }
}

export class AeroVisualization {
    constructor(scene, options = {}) {
        this.scene = scene;
        this.particleCount = options.particleCount || 200;
        this.streamlineCount = options.streamlineCount || 18;
        this.maxAge = options.maxAge || 250;
        this.windSpeed = 5;
        this.windDirection = 0;
        this.isStalled = false;
        this.sailData = options.sailData || [];

        this.sailPositions = [
            { x: -8, y: 8, z: 0, height: 15, width: 10, span: 10 },
            { x: 0, y: 10, z: 0, height: 18, width: 12, span: 12 },
            { x: 10, y: 7, z: 0, height: 13, width: 8, span: 8 }
        ];

        this.vorticityField = {
            gridX: 30, gridY: 20, gridZ: 20,
            bounds: { minX: -15, maxX: 25, minY: -5, maxY: 25, minZ: -15, maxZ: 15 },
            data: new Float32Array(30 * 20 * 20),
            maxVorticity: 0.01
        };
        this.vorticityCache = null;
        this.createVorticityField();
        this.updateVorticityField();

        this.createParticleSystem();
        this.createStreamlines();
        this.vortexLines = new THREE.Group();
        this.scene.add(this.vortexLines);
        this.updateVortexLines();
    }

    createVorticityField() {
        const dx = (this.vorticityField.bounds.maxX - this.vorticityField.bounds.minX) / (this.vorticityField.gridX - 1);
        const dy = (this.vorticityField.bounds.maxY - this.vorticityField.bounds.minY) / (this.vorticityField.gridY - 1);
        const dz = (this.vorticityField.bounds.maxZ - this.vorticityField.bounds.minZ) / (this.vorticityField.gridZ - 1);
        this.vorticityField.cellSize = Math.min(dx, dy, dz);
    }

    updateVorticityField() {
        this.sailPositions.forEach(sail => {
            this.vorticityField.maxVorticity = Math.max(0.01, 1.2 * Math.PI * sail.span * this.windSpeed / (sail.height * sail.height + 1e-6));
        });
        const idx = (x, y, z) => (x * this.vorticityField.gridY + y) * this.vorticityField.gridZ + z;
        for (let x = 0; x < this.vorticityField.gridX; x++) {
            for (let y = 0; y < this.vorticityField.gridY; y++) {
                for (let z = 0; z < this.vorticityField.gridZ; z++) {
                    const wx = this.vorticityField.bounds.minX + x * this.vorticityField.cellSize;
                    const wy = this.vorticityField.bounds.minY + y * this.vorticityField.cellSize;
                    const wz = this.vorticityField.bounds.minZ + z * this.vorticityField.cellSize;
                    this.vorticityField.data[idx(x, y, z)] = this.computeVorticityMagnitude(wx, wy, wz);
                }
            }
        }
    }

    computeVorticityMagnitude(x, y, z) {
        let totalVort = 0;
        const windRad = this.windDirection * Math.PI / 180;
        const windU = Math.cos(windRad) * this.windSpeed;
        const windV = Math.sin(windRad) * this.windSpeed;

        this.sailPositions.forEach((sail, sIdx) => {
            const halfSpan = sail.span / 2;
            const halfHeight = sail.height / 2;
            const tip1 = { x: sail.x + halfSpan * Math.cos(windRad), y: sail.y + halfHeight, z: sail.z + halfSpan * Math.sin(windRad) };
            const tip2 = { x: sail.x - halfSpan * Math.cos(windRad), y: sail.y + halfHeight, z: sail.z - halfSpan * Math.sin(windRad) };
            const circulation = 0.5 * this.windSpeed * sail.height * sail.height / Math.max(sail.span, 0.1);

            [tip1, tip2].forEach(tip => {
                const dx = x - tip.x;
                const dy = y - tip.y;
                const dz = z - tip.z;
                const r2 = dx * dx + dy * dy + dz * dz;
                const r = Math.sqrt(r2);
                const coreRadius = 0.5;
                const factor = circulation / (2 * Math.PI * Math.max(r2, coreRadius * coreRadius));
                const crossX = -dz;
                const crossZ = dx;
                totalVort += factor * Math.sqrt(crossX * crossX + crossZ * crossZ) * Math.exp(-r / (3 * coreRadius));
            });

            const dx = x - sail.x;
            const dy = y - sail.y;
            const dz = z - sail.z;
            const dist = Math.sqrt(dx * dx + dy * dy + dz * dz);
            const sheetVort = (1.5 * this.windSpeed / sail.height) * Math.exp(-dist / 1.2);
            totalVort += sheetVort;
        });

        if (this.isStalled) {
            const avgX = this.sailPositions.reduce((s, p) => s + p.x, 0) / this.sailPositions.length;
            const avgY = this.sailPositions.reduce((s, p) => s + p.y, 0) / this.sailPositions.length;
            const avgZ = this.sailPositions.reduce((s, p) => s + p.z, 0) / this.sailPositions.length;
            const dx = x - avgX;
            const dy = y - avgY;
            const dz = z - avgZ;
            const r = Math.sqrt(dx * dx + dy * dy + dz * dz);
            const shearVort = (2.5 * this.windSpeed / 5) * Math.exp(-r / 5) * (0.8 + 0.2 * Math.sin(r * 2));
            totalVort += Math.abs(shearVort);
        }

        return totalVort;
    }

    sampleVorticityCDF() {
        const samples = 200;
        const cdf = new Array(samples);
        const minX = this.vorticityField.bounds.minX;
        const maxX = this.vorticityField.bounds.maxX;
        const midY = (this.vorticityField.bounds.minY + this.vorticityField.bounds.maxY) / 2;
        const midZ = (this.vorticityField.bounds.minZ + this.vorticityField.bounds.maxZ) / 2;

        let sum = 0;
        for (let i = 0; i < samples; i++) {
            const x = minX + (maxX - minX) * (i / (samples - 1));
            const v = this.computeVorticityMagnitude(x, midY, midZ);
            sum += Math.max(0, v);
            cdf[i] = sum;
        }
        if (sum > 0) {
            for (let i = 0; i < samples; i++) {
                cdf[i] /= sum;
            }
        }
        return { cdf, minX, maxX, midY, midZ };
    }

    seedByVorticity() {
        const { cdf, minX, maxX, midY, midZ } = this.sampleVorticityCDF();
        const r = Math.random();
        let lo = 0, hi = cdf.length - 1;
        while (lo < hi) {
            const mid = (lo + hi) >> 1;
            if (cdf[mid] < r) lo = mid + 1;
            else hi = mid;
        }
        const t = Math.max(0, Math.min(1, (lo + (Math.random() - 0.5) * 0.2) / (cdf.length - 1)));
        return {
            x: minX + (maxX - minX) * t,
            y: midY + (Math.random() - 0.5) * 10,
            z: midZ + (Math.random() - 0.5) * 10
        };
    }

    createParticleSystem() {
        const geometry = new THREE.BufferGeometry();
        const positions = new Float32Array(this.particleCount * 3);
        const colors = new Float32Array(this.particleCount * 3);
        const velocities = new Float32Array(this.particleCount * 3);
        const ages = new Float32Array(this.particleCount);
        const lifetimes = new Float32Array(this.particleCount);

        for (let i = 0; i < this.particleCount; i++) {
            if (Math.random() < 0.65) {
                const pos = this.seedByVorticity();
                positions[i * 3] = pos.x;
                positions[i * 3 + 1] = pos.y;
                positions[i * 3 + 2] = pos.z;
            } else {
                positions[i * 3] = this.vorticityField.bounds.minX + Math.random() * (this.vorticityField.bounds.maxX - this.vorticityField.bounds.minX);
                positions[i * 3 + 1] = this.vorticityField.bounds.minY + Math.random() * (this.vorticityField.bounds.maxY - this.vorticityField.bounds.minY);
                positions[i * 3 + 2] = this.vorticityField.bounds.minZ + Math.random() * (this.vorticityField.bounds.maxZ - this.vorticityField.bounds.minZ);
            }
            const color = new THREE.Color().setHSL(0.55 + Math.random() * 0.1, 0.8, 0.5 + Math.random() * 0.2);
            colors[i * 3] = color.r;
            colors[i * 3 + 1] = color.g;
            colors[i * 3 + 2] = color.b;
            velocities[i * 3] = 0;
            velocities[i * 3 + 1] = 0;
            velocities[i * 3 + 2] = 0;
            ages[i] = Math.random() * this.maxAge;
            lifetimes[i] = this.maxAge + Math.random() * 100;
        }

        geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
        geometry.setAttribute('velocity', new THREE.BufferAttribute(velocities, 3));

        const material = new THREE.PointsMaterial({
            size: 0.25,
            vertexColors: true,
            transparent: true,
            opacity: 0.75,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });

        this.particles = new THREE.Points(geometry, material);
        this.particleAges = ages;
        this.particleLifetimes = lifetimes;
        this.scene.add(this.particles);
    }

    resetParticleVorticity(i) {
        const positions = this.particles.geometry.attributes.position.array;
        const colors = this.particles.geometry.attributes.color.array;
        if (Math.random() < 0.65) {
            const pos = this.seedByVorticity();
            positions[i * 3] = pos.x;
            positions[i * 3 + 1] = pos.y;
            positions[i * 3 + 2] = pos.z;
        } else {
            positions[i * 3] = this.vorticityField.bounds.minX - 2;
            positions[i * 3 + 1] = this.vorticityField.bounds.minY + Math.random() * (this.vorticityField.bounds.maxY - this.vorticityField.bounds.minY);
            positions[i * 3 + 2] = this.vorticityField.bounds.minZ + Math.random() * (this.vorticityField.bounds.maxZ - this.vorticityField.bounds.minZ);
        }
        const hue = this.isStalled ? 0.08 : 0.55 + Math.random() * 0.1;
        const color = new THREE.Color().setHSL(hue, 0.8, 0.5 + Math.random() * 0.2);
        colors[i * 3] = color.r;
        colors[i * 3 + 1] = color.g;
        colors[i * 3 + 2] = color.b;
        this.particleAges[i] = 0;
        this.particleLifetimes[i] = this.maxAge + Math.random() * 100;
    }

    createStreamlines() {
        this.streamlines = [];
        for (let i = 0; i < this.streamlineCount; i++) {
            const points = [];
            const startX = this.vorticityField.bounds.minX;
            const y = this.vorticityField.bounds.minY + (this.vorticityField.bounds.maxY - this.vorticityField.bounds.minY) * ((i + 0.5) / this.streamlineCount);
            const z = this.vorticityField.bounds.minZ + (this.vorticityField.bounds.maxZ - this.vorticityField.bounds.minZ) * (0.3 + 0.4 * ((i + 0.5) / this.streamlineCount));
            for (let j = 0; j < 100; j++) {
                points.push(new THREE.Vector3(startX + j * 0.5, y, z));
            }
            const geometry = new THREE.BufferGeometry().setFromPoints(points);
            const hue = 0.55 + (i / this.streamlineCount) * 0.1;
            const material = new THREE.LineBasicMaterial({
                color: new THREE.Color().setHSL(hue, 0.7, 0.5),
                transparent: true,
                opacity: 0.4
            });
            const line = new THREE.Line(geometry, material);
            this.streamlines.push(line);
            this.scene.add(line);
        }
    }

    setStall(stalled) {
        if (this.isStalled !== stalled) {
            this.isStalled = stalled;
            this.updateVorticityField();
            const colors = this.particles.geometry.attributes.color.array;
            for (let i = 0; i < this.particleCount; i++) {
                if (this.isStalled && Math.random() < 0.5) {
                    const c = new THREE.Color().setHSL(0.08, 0.8, 0.5);
                    colors[i * 3] = c.r;
                    colors[i * 3 + 1] = c.g;
                    colors[i * 3 + 2] = c.b;
                }
            }
            this.particles.geometry.attributes.color.needsUpdate = true;
            this.updateVortexLines();
        }
    }

    setWind(speed, directionDeg) {
        this.windSpeed = speed;
        this.windDirection = directionDeg;
        this.updateVorticityField();
    }

    update(deltaTime) {
        if (this.particles) {
            const positions = this.particles.geometry.attributes.position.array;
            const velocities = this.particles.geometry.attributes.velocity.array;
            for (let i = 0; i < this.particleCount; i++) {
                this.particleAges[i]++;
                if (this.particleAges[i] >= this.particleLifetimes[i]) {
                    this.resetParticleVorticity(i);
                    continue;
                }
                const x = positions[i * 3];
                const y = positions[i * 3 + 1];
                const z = positions[i * 3 + 2];
                const velocity = this.calculateVelocity(x, y, z);
                const vort = this.computeVorticityMagnitude(x, y, z);
                const vortNorm = Math.min(vort / (this.vorticityField.maxVorticity || 1), 1);
                const step = this.adaptiveStepSize(vortNorm);
                positions[i * 3] += velocity.x * step;
                positions[i * 3 + 1] += velocity.y * step;
                positions[i * 3 + 2] += velocity.z * step;
                if (positions[i * 3] > this.vorticityField.bounds.maxX + 5 ||
                    positions[i * 3] < this.vorticityField.bounds.minX - 5 ||
                    positions[i * 3 + 1] < this.vorticityField.bounds.minY - 5 ||
                    positions[i * 3 + 1] > this.vorticityField.bounds.maxY + 5 ||
                    Math.abs(positions[i * 3 + 2]) > this.vorticityField.bounds.maxZ + 5) {
                    this.resetParticleVorticity(i);
                }
            }
            this.particles.geometry.attributes.position.needsUpdate = true;
        }

        this.streamlines.forEach((line, idx) => {
            this.updateStreamline(line, idx);
        });
    }

    adaptiveStepSize(vortNorm) {
        const baseStep = 0.7;
        return Math.max(0.15, baseStep * (1 - 0.7 * vortNorm));
    }

    calculateVelocity(x, y, z) {
        const windRad = this.windDirection * Math.PI / 180;
        let vx = Math.cos(windRad) * this.windSpeed;
        let vy = 0;
        let vz = Math.sin(windRad) * this.windSpeed;
        const disturb = this.calculateSailDisturbance(x, y, z);
        vx += disturb.x;
        vy += disturb.y;
        vz += disturb.z;
        if (this.isStalled) {
            const turb = this.calculateStallTurbulence(x, y, z);
            vx += turb.x;
            vy += turb.y;
            vz += turb.z;
        }
        return { x: vx, y: vy, z: vz };
    }

    calculateSailDisturbance(x, y, z) {
        let dx = 0, dy = 0, dz = 0;
        this.sailPositions.forEach(sail => {
            const sx = x - sail.x;
            const sy = y - sail.y;
            const sz = z - sail.z;
            const dist = Math.sqrt(sx * sx + sy * sy + sz * sz);
            if (dist < sail.height && dist > 0.5) {
                const upwash = sail.height / (dist * dist + sail.height * sail.height) * this.windSpeed * 0.3;
                dy += upwash;
                const factor = Math.exp(-dist / 3) * this.windSpeed * 0.2;
                dx += (sx / dist) * factor;
                dz += (sz / dist) * factor;
            }
        });
        return { x: dx, y: dy, z: dz };
    }

    calculateStallTurbulence(x, y, z) {
        const time = Date.now() * 0.001;
        return {
            x: Math.sin(time * 2 + x) * this.windSpeed * 0.15,
            y: Math.cos(time * 1.5 + y) * this.windSpeed * 0.1,
            z: Math.sin(time * 1.8 + z) * this.windSpeed * 0.15
        };
    }

    updateStreamline(line, index) {
        const positions = line.geometry.attributes.position.array;
        if (positions.length < 3) return;

        const startX = this.vorticityField.bounds.minX;
        const y = this.vorticityField.bounds.minY + (this.vorticityField.bounds.maxY - this.vorticityField.bounds.minY) * ((index + 0.5) / this.streamlineCount);
        const z = this.vorticityField.bounds.minZ + (this.vorticityField.bounds.maxZ - this.vorticityField.bounds.minZ) * (0.3 + 0.4 * ((index + 0.5) / this.streamlineCount));

        let currX = startX;
        let currY = y;
        let currZ = z;

        for (let i = 0; i < positions.length / 3; i++) {
            positions[i * 3] = currX;
            positions[i * 3 + 1] = currY;
            positions[i * 3 + 2] = currZ;
            const velocity = this.calculateVelocity(currX, currY, currZ);
            const vort = this.computeVorticityMagnitude(currX, currY, currZ);
            const vortNorm = Math.min(vort / (this.vorticityField.maxVorticity || 1), 1);
            const step = this.adaptiveStepSize(vortNorm);
            currX += velocity.x * step;
            currY += velocity.y * step;
            currZ += velocity.z * step;
        }

        line.geometry.attributes.position.needsUpdate = true;
    }

    updateVortexLines() {
        while (this.vortexLines.children.length > 0) {
            const child = this.vortexLines.children[0];
            this.vortexLines.remove(child);
            child.geometry.dispose();
            child.material.dispose();
        }

        if (!this.isStalled) return;

        const ringCount = 12;
        this.sailPositions.forEach((sail, sIdx) => {
            for (let rIdx = 0; rIdx < ringCount; rIdx++) {
                const radius = 1 + (rIdx / ringCount) * 4;
                const segments = 32;
                const points = [];
                for (let i = 0; i <= segments; i++) {
                    const angle = (i / segments) * Math.PI * 2;
                    const x = sail.x + 0.3 * sail.width + radius * Math.cos(angle) * 0.8;
                    const y = sail.y + radius * Math.sin(angle);
                    const z = sail.z + radius * Math.cos(angle) * 0.3;
                    points.push(new THREE.Vector3(x, y, z));
                }
                const geometry = new THREE.BufferGeometry().setFromPoints(points);
                const hue = 0.05 + Math.random() * 0.05;
                const material = new THREE.LineBasicMaterial({
                    color: new THREE.Color().setHSL(hue, 0.9, 0.5 + Math.random() * 0.2),
                    transparent: true,
                    opacity: 0.5
                });
                const line = new THREE.Line(geometry, material);
                this.vortexLines.add(line);
            }
        });
    }
}

export function setup3DScene(container) {
    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x0a1628);
    scene.fog = new THREE.FogExp2(0x0a1628, 0.008);

    const camera = new THREE.PerspectiveCamera(55, container.clientWidth / container.clientHeight, 0.1, 1000);
    camera.position.set(25, 15, 35);

    const renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setSize(container.clientWidth, container.clientHeight);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.2;
    container.appendChild(renderer.domElement);

    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.05;
    controls.maxPolarAngle = Math.PI / 2.1;
    controls.minDistance = 15;
    controls.maxDistance = 100;
    controls.target.set(0, 5, 0);

    const ambientLight = new THREE.AmbientLight(0x404040, 0.4);
    scene.add(ambientLight);

    const sunLight = new THREE.DirectionalLight(0xfff5e6, 1.0);
    sunLight.position.set(30, 40, 20);
    sunLight.castShadow = true;
    sunLight.shadow.mapSize.width = 2048;
    sunLight.shadow.mapSize.height = 2048;
    sunLight.shadow.camera.near = 0.5;
    sunLight.shadow.camera.far = 100;
    sunLight.shadow.camera.left = -40;
    sunLight.shadow.camera.right = 40;
    sunLight.shadow.camera.top = 40;
    sunLight.shadow.camera.bottom = -40;
    scene.add(sunLight);

    const fillLight = new THREE.DirectionalLight(0x87ceeb, 0.3);
    fillLight.position.set(-20, 10, -20);
    scene.add(fillLight);

    const hemiLight = new THREE.HemisphereLight(0x87ceeb, 0x1a4c6e, 0.3);
    scene.add(hemiLight);

    return { scene, camera, renderer, controls };
}
