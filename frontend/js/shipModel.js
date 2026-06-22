import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

export class ShipModel {
    constructor(scene, sailData) {
        this.scene = scene;
        this.sailData = sailData;
        this.shipGroup = new THREE.Group();
        this.sailMeshes = [];
        this.sailGeometries = [];
        this.hullMesh = null;
        this.mastMeshes = [];
        this.animateSails = true;
        this.time = 0;

        this.buildHull();
        this.buildMasts();
        this.buildSails();
        this.buildRigging();
        this.buildSea();

        this.scene.add(this.shipGroup);
    }

    buildHull() {
        const hullGroup = new THREE.Group();
        const hullLength = 30;
        const hullWidth = 10;
        const hullHeight = 6;

        const hullShape = new THREE.Shape();
        hullShape.moveTo(-hullLength / 2, 0);
        hullShape.quadraticCurveTo(-hullLength / 2, hullHeight / 3, -hullLength / 2 + 3, hullHeight / 2);
        hullShape.lineTo(hullLength / 2 - 5, hullHeight / 2);
        hullShape.quadraticCurveTo(hullLength / 2, hullHeight / 3, hullLength / 2, 0);
        hullShape.quadraticCurveTo(hullLength / 2 - 3, -hullHeight / 6, hullLength / 3, -hullHeight / 8);
        hullShape.lineTo(-hullLength / 3, -hullHeight / 8);
        hullShape.quadraticCurveTo(-hullLength / 2 + 3, -hullHeight / 6, -hullLength / 2, 0);

        const extrudeSettings = {
            steps: 24,
            depth: hullWidth,
            bevelEnabled: true,
            bevelThickness: 0.3,
            bevelSize: 0.2,
            bevelSegments: 4,
        };

        const hullGeometry = new THREE.ExtrudeGeometry(hullShape, extrudeSettings);
        hullGeometry.center();
        hullGeometry.rotateY(Math.PI / 2);
        hullGeometry.rotateZ(Math.PI);

        const hullMaterial = new THREE.MeshStandardMaterial({
            color: 0x8B4513,
            roughness: 0.8,
            metalness: 0.1,
            side: THREE.DoubleSide,
        });

        this.hullMesh = new THREE.Mesh(hullGeometry, hullMaterial);
        this.hullMesh.castShadow = true;
        this.hullMesh.receiveShadow = true;
        hullGroup.add(this.hullMesh);

        const deckGeometry = new THREE.BoxGeometry(hullLength * 0.8, 0.3, hullWidth * 0.9);
        const deckMaterial = new THREE.MeshStandardMaterial({
            color: 0x654321,
            roughness: 0.9,
        });
        const deck = new THREE.Mesh(deckGeometry, deckMaterial);
        deck.position.y = hullHeight / 2 + 0.1;
        deck.castShadow = true;
        hullGroup.add(deck);

        const cabinGeometry = new THREE.BoxGeometry(5, 2.5, 4);
        const cabinMaterial = new THREE.MeshStandardMaterial({
            color: 0x5D4037,
            roughness: 0.85,
        });
        const cabin = new THREE.Mesh(cabinGeometry, cabinMaterial);
        cabin.position.set(-hullLength / 4, hullHeight / 2 + 1.5, 0);
        cabin.castShadow = true;
        hullGroup.add(cabin);

        const roofGeometry = new THREE.ConeGeometry(3.5, 1.2, 4);
        const roofMaterial = new THREE.MeshStandardMaterial({
            color: 0x3E2723,
            roughness: 0.9,
        });
        const roof = new THREE.Mesh(roofGeometry, roofMaterial);
        roof.position.set(-hullLength / 4, hullHeight / 2 + 3.3, 0);
        roof.rotateY(Math.PI / 4);
        hullGroup.add(roof);

        this.shipGroup.add(hullGroup);
    }

    buildMasts() {
        const mastPositions = [
            { x: 2, height: 25, name: 'main' },
            { x: 9, height: 20, name: 'fore' },
            { x: -8, height: 16, name: 'mizzen' },
        ];

        const mastMaterial = new THREE.MeshStandardMaterial({
            color: 0x4E342E,
            roughness: 0.7,
        });

        mastPositions.forEach((pos, idx) => {
            const mastGeometry = new THREE.CylinderGeometry(0.25, 0.35, pos.height, 12);
            const mast = new THREE.Mesh(mastGeometry, mastMaterial);
            mast.position.set(pos.x, 3 + pos.height / 2, 0);
            mast.castShadow = true;
            this.shipGroup.add(mast);
            this.mastMeshes.push(mast);

            const yardGeometry = new THREE.CylinderGeometry(0.1, 0.1, 14, 8);
            const yardMaterial = new THREE.MeshStandardMaterial({
                color: 0x6D4C41,
                roughness: 0.75,
            });
            const yard = new THREE.Mesh(yardGeometry, yardMaterial);
            yard.rotation.z = Math.PI / 2;
            yard.position.set(pos.x, 3 + pos.height * 0.6, 0);
            yard.castShadow = true;
            this.shipGroup.add(yard);

            if (idx === 0) {
                const flagGeometry = new THREE.PlaneGeometry(2.5, 1.5);
                const flagMaterial = new THREE.MeshStandardMaterial({
                    color: 0xD32F2F,
                    side: THREE.DoubleSide,
                });
                const flag = new THREE.Mesh(flagGeometry, flagMaterial);
                flag.position.set(pos.x, 3 + pos.height + 0.5, 1.2);
                this.shipGroup.add(flag);
            }
        });
    }

    buildSails() {
        const sailConfigs = [
            { x: 2, mastHeight: 25, width: 12, height: 16, yBase: 5, camber: 0.13, name: '主桅帆' },
            { x: 9, mastHeight: 20, width: 10, height: 13, yBase: 5, camber: 0.12, name: '前桅帆' },
            { x: -8, mastHeight: 16, width: 8, height: 11, yBase: 4.5, camber: 0.11, name: '后桅帆' },
        ];

        const sailColors = [0xF5E6C8, 0xE8D5A9, 0xDFC98D];

        sailConfigs.forEach((config, idx) => {
            const segmentsU = 20;
            const segmentsV = 25;

            const geometry = new THREE.PlaneGeometry(config.width, config.height, segmentsU, segmentsV);
            this.sailGeometries.push({ geo: geometry, config, idx });

            const material = new THREE.MeshStandardMaterial({
                color: sailColors[idx % sailColors.length],
                side: THREE.DoubleSide,
                roughness: 0.95,
                metalness: 0.0,
                transparent: true,
                opacity: 0.92,
            });

            const sail = new THREE.Mesh(geometry, material);
            sail.position.set(config.x, config.yBase + config.height / 2, 0);
            sail.castShadow = true;
            sail.receiveShadow = true;

            this.sailMeshes.push({
                mesh: sail,
                geometry,
                config,
                segmentsU,
                segmentsV,
                baseAngle: 0,
            });

            this.shipGroup.add(sail);

            this.updateSailShape(geometry, config, 0, 0, config.camber);
        });
    }

    updateSailShape(geometry, config, angle, windStrength, camber) {
        const positions = geometry.attributes.position;
        const segmentsU = geometry.parameters.widthSegments;
        const segmentsV = geometry.parameters.heightSegments;
        const width = config.width;
        const height = config.height;

        const angleRad = angle * Math.PI / 180;
        const effectiveCamber = camber * (1 + windStrength * 0.3);

        for (let i = 0; i <= segmentsV; i++) {
            for (let j = 0; j <= segmentsU; j++) {
                const idx = i * (segmentsU + 1) + j;

                const u = j / segmentsU;
                const v = i / segmentsV;

                const xLocal = (u - 0.5) * width;
                const yLocal = (v - 0.5) * height;

                const taperFactor = 1 - Math.pow(v - 0.5, 2) * 0.3;
                const xTapered = xLocal * taperFactor;

                const camberProfile = 4 * effectiveCamber * (u - u * u);
                const tensionFactor = Math.sin(v * Math.PI);
                const zDisplacement = camberProfile * height * 0.3 * tensionFactor;

                const xRot = xTapered * Math.cos(angleRad) + zDisplacement * Math.sin(angleRad);
                const zRot = -xTapered * Math.sin(angleRad) + zDisplacement * Math.cos(angleRad);

                positions.setXYZ(idx, xRot, yLocal, zRot);
            }
        }

        geometry.attributes.position.needsUpdate = true;
        geometry.computeVertexNormals();
    }

    buildRigging() {
        const riggingMaterial = new THREE.LineBasicMaterial({
            color: 0x4A4A4A,
            linewidth: 1,
        });

        const mastPositions = [
            { x: 2, h: 28 },
            { x: 9, h: 23 },
            { x: -8, h: 19 },
        ];

        mastPositions.forEach((mast) => {
            const points1 = [
                new THREE.Vector3(mast.x, 3 + mast.h, 0),
                new THREE.Vector3(-15, 6, -5),
            ];
            const points2 = [
                new THREE.Vector3(mast.x, 3 + mast.h, 0),
                new THREE.Vector3(-15, 6, 5),
            ];
            const points3 = [
                new THREE.Vector3(mast.x, 3 + mast.h, 0),
                new THREE.Vector3(15, 6, -5),
            ];
            const points4 = [
                new THREE.Vector3(mast.x, 3 + mast.h, 0),
                new THREE.Vector3(15, 6, 5),
            ];

            [points1, points2, points3, points4].forEach((pts) => {
                const g = new THREE.BufferGeometry().setFromPoints(pts);
                const line = new THREE.Line(g, riggingMaterial);
                this.shipGroup.add(line);
            });
        });
    }

    buildSea() {
        const seaGeometry = new THREE.PlaneGeometry(200, 200, 50, 50);
        const seaMaterial = new THREE.MeshStandardMaterial({
            color: 0x0e4d6c,
            transparent: true,
            opacity: 0.85,
            roughness: 0.3,
            metalness: 0.1,
            side: THREE.DoubleSide,
        });

        const sea = new THREE.Mesh(seaGeometry, seaMaterial);
        sea.rotation.x = -Math.PI / 2;
        sea.position.y = -2;
        sea.receiveShadow = true;
        this.seaMesh = sea;
        this.seaGeometry = seaGeometry;
        this.scene.add(sea);
    }

    setSailAngle(sailIndex, angle) {
        if (sailIndex >= 0 && sailIndex < this.sailMeshes.length) {
            this.sailMeshes[sailIndex].baseAngle = angle;
        }
    }

    setShipHeading(heading) {
        this.shipGroup.rotation.y = -heading * Math.PI / 180;
    }

    update(deltaTime, windSpeed, windDirection, windAngleRel) {
        this.time += deltaTime;

        if (this.animateSails) {
            this.sailMeshes.forEach((sail, idx) => {
                const wave = Math.sin(this.time * 2 + idx) * 2;
                const gust = windSpeed > 8 ? Math.sin(this.time * 5) * 0.5 : 0;
                const windFactor = Math.min(windSpeed / 15, 1);

                const totalAngle = sail.baseAngle + wave * 0.3 + gust;
                this.updateSailShape(
                    sail.geometry,
                    sail.config,
                    totalAngle,
                    windFactor,
                    sail.config.camber,
                );
            });
        }

        if (this.seaGeometry) {
            const positions = this.seaGeometry.attributes.position;
            for (let i = 0; i < positions.count; i++) {
                const x = positions.getX(i);
                const y = positions.getY(i);
                const wave1 = Math.sin(x * 0.05 + this.time * 0.8) * 0.4;
                const wave2 = Math.sin(y * 0.07 + this.time * 0.6) * 0.25;
                positions.setZ(i, wave1 + wave2);
            }
            positions.needsUpdate = true;
            this.seaGeometry.computeVertexNormals();
        }

        const roll = Math.sin(this.time * 0.4) * 0.02 + (windSpeed > 10 ? windAngleRel * 0.001 : 0);
        const pitch = Math.sin(this.time * 0.3) * 0.015;
        this.shipGroup.rotation.z = roll;
        this.shipGroup.rotation.x = pitch;
        this.shipGroup.position.y = Math.sin(this.time * 0.5) * 0.1;
    }

    getSailWorldPosition(sailIndex) {
        if (sailIndex >= this.sailMeshes.length) return null;
        const pos = new THREE.Vector3();
        this.sailMeshes[sailIndex].mesh.getWorldPosition(pos);
        return pos;
    }
}
