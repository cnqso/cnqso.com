import { sleep, initCanvas } from "./canvas.js";
import type { Coord } from "./canvas.js";

const RIPPLE_SPEED = 50;

type Axial = { q: number; r: number }; // s is implied: s = -q - r
type Cube = { x: number; y: number; z: number };

function cubeToAxial(c: Cube): Axial {
    return { q: c.x, r: c.z };
}

const DIRS: Axial[] = [
    { q: +1, r: 0 },
    { q: +1, r: -1 },
    { q: 0, r: -1 },
    { q: -1, r: 0 },
    { q: -1, r: +1 },
    { q: 0, r: +1 },
];
function addAxial(a: Axial, b: Axial): Axial {
    return { q: a.q + b.q, r: a.r + b.r };
}
function neighbors(a: Axial): Axial[] {
    return DIRS.map((d) => addAxial(a, d));
}

function axialToPixel(a: Axial, radius: number): Coord {
    const x = radius * (1.5 * a.q);
    const y = radius * (Math.sqrt(3) * (a.r + a.q / 2));
    return [x, y];
}

function pixelToAxial(px: Coord, radius: number): Axial {
    const [x, y] = px;
    const qf = ((2 / 3) * x) / radius;
    const rf = ((-1 / 3) * x + (Math.sqrt(3) / 3) * y) / radius;
    return cubeToAxial(cubeRound({ x: qf, y: -qf - rf, z: rf }));
}

function cubeDistance(a: Axial, b: Axial): number {
    const ax = a.q,
        ay = -a.q - a.r,
        az = a.r;
    const bx = b.q,
        by = -b.q - b.r,
        bz = b.r;

    return (Math.abs(ax - bx) + Math.abs(ay - by) + Math.abs(az - bz)) / 2;
}

function cubeRound(frac: Cube): Cube {
    let rx = Math.round(frac.x);
    let ry = Math.round(frac.y);
    let rz = Math.round(frac.z);

    const dx = Math.abs(rx - frac.x);
    const dy = Math.abs(ry - frac.y);
    const dz = Math.abs(rz - frac.z);

    if (dx > dy && dx > dz) {
        rx = -ry - rz;
    } else if (dy > dz) {
        ry = -rx - rz;
    } else {
        rz = -rx - ry;
    }
    return { x: rx, y: ry, z: rz };
}

function randomColor() {
    const r = () => Math.floor(Math.random() * 255);
    return `rgb(${r()} ${r()} ${r()})`;
}

class Hexagon {
    axial: Axial;
    radius: number;
    color: string;

    constructor(axial: Axial, radius: number, color: string) {
        this.axial = axial;
        this.radius = radius;
        this.color = color;
    }

    get center(): Coord {
        return axialToPixel(this.axial, this.radius);
    }

    vertices(): Coord[] {
        const [cx, cy] = this.center;
        const verts: Coord[] = [];
        for (let i = 0; i < 6; i++) {
            const angle = i * (Math.PI / 3);
            const x = cx + this.radius * Math.cos(angle);
            const y = cy + this.radius * Math.sin(angle);
            verts.push([x, y]);
        }
        return verts;
    }
}

function rectangleAxial(width: number, height: number): Axial[] {
    const results: Axial[] = [];
    const qmin = -Math.floor(width / 2),
        qmax = Math.floor(width / 2);
    const rmin = -Math.floor(height / 2),
        rmax = Math.floor(height / 2);
    for (let q = qmin; q <= qmax; q++) {
        for (let r = rmin; r <= rmax; r++) {
            results.push({ q, r });
        }
    }
    return results;
}

async function hexagons() {
    const canvas = initCanvas();
    if (!canvas) {
        console.error("Canvas failed to initialize");
        return;
    }
    canvas.style.cursor = "pointer";
    const ctx = canvas.getContext("2d")!;
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";

    const radius = 30;

    const axialField = rectangleAxial(100, 60);
    const hexArray = axialField.map(
        (a) => new Hexagon(a, radius, randomColor()),
    );

    const hexIndex = new Map<string, Hexagon>();
    for (const h of hexArray) hexIndex.set(`${h.axial.q},${h.axial.r}`, h);

    let hoveredKey: string | null = null;

    let rippleActive = false;
    let rippleOrigin: Axial | null = null;
    let rippleStart = 0;
    let rippleColor = "red";
    let maxRippleDist = 0;

    function drawHexagon(h: Hexagon, oW: number, oH: number, fill: string) {
        const verts = h.vertices();
        ctx.beginPath();
        ctx.moveTo(verts[0][0] + oW, verts[0][1] + oH);
        for (let i = 1; i < 6; i++)
            ctx.lineTo(verts[i][0] + oW, verts[i][1] + oH);
        ctx.closePath();
        ctx.fillStyle = fill;
        ctx.fill();
    }

    canvas.addEventListener("mousemove", (e) => {
        const rect = canvas.getBoundingClientRect();
        const mx = e.clientX - rect.left - canvas.width / 2;
        const my = e.clientY - rect.top - canvas.height / 2;
        const axial = pixelToAxial([mx, my], radius);
        const k = `${axial.q},${axial.r}`;
        hoveredKey = hexIndex.has(k) ? k : null;
    });
    canvas.addEventListener("click", (e) => {
        const rect = canvas.getBoundingClientRect();
        const mx = e.clientX - rect.left - canvas.width / 2;
        const my = e.clientY - rect.top - canvas.height / 2;
        const axial = pixelToAxial([mx, my], radius);

        const k = `${axial.q},${axial.r}`;
        if (!hexIndex.has(k)) return;

        rippleActive = true;
        rippleOrigin = axial;
        rippleStart = performance.now();
        rippleColor = randomColor();

        maxRippleDist = 0;
        for (const h of hexArray) {
            const d = cubeDistance(axial, h.axial);
            if (d > maxRippleDist) maxRippleDist = d;
        }
    });

    async function render() {
        while (true) {
            const oW = canvas.width / 2;
            const oH = canvas.height / 2;
            ctx.clearRect(0, 0, canvas.width, canvas.height);

            let currentRing = -1;
            if (rippleActive && rippleOrigin) {
                const elapsed = (performance.now() - rippleStart) / 1000;
                currentRing = Math.floor(elapsed * RIPPLE_SPEED);
            }

            for (const h of hexArray) {
                if (rippleActive && rippleOrigin) {
                    const dist = cubeDistance(rippleOrigin, h.axial);
                    if (dist <= currentRing) {
                        h.color = rippleColor;
                    }
                }
                const k = `${h.axial.q},${h.axial.r}`;
                const isHover = hoveredKey && hoveredKey === k;
                const fill = isHover ? "red" : h.color;
                drawHexagon(h, oW, oH, fill);
            }

            if (rippleActive && currentRing > maxRippleDist) {
                rippleActive = false;
                rippleOrigin = null;
            }

            await sleep(9);
        }
    }

    render();
}

window.onload = async function () {
    await hexagons();
};
