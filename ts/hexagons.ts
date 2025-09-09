import { sleep, initCanvas } from "./canvas.js";
import type { Coord } from "./canvas.js";

type Axial = { q: number; r: number }; // s is implied: s = -q - r
type Cube = { x: number; y: number; z: number };

const HEXINDEX = new Map<string, Hexagon>();
let RESET = false;
let outlinePath: Path2D | null = null;

function buildOutlinePath(hexagons: Hexagon[]): Path2D {
  const path = new Path2D();
  for (const hex of hexagons) {
    path.addPath(hex.path);
  }
  return path;
}

function drawOutlines(ctx: CanvasRenderingContext2D, canvas: HTMLCanvasElement, hexagons: Hexagon[]) {
  if (!outlinePath) {
    outlinePath = buildOutlinePath(hexagons);
  }

  const oW = canvas.width / 2;
  const oH = canvas.height / 2;
  ctx.save();
  ctx.translate(oW, oH);
  ctx.strokeStyle = "#7c7f93";
  ctx.lineWidth = 1;
  ctx.stroke(outlinePath);
  ctx.restore();
}

const DIRS: Axial[] = [
  { q: +1, r: 0 },
  { q: +1, r: -1 },
  { q: 0, r: -1 },
  { q: -1, r: 0 },
  { q: -1, r: +1 },
  { q: 0, r: +1 },
];



const CONFIG = {
  bgColor: "#ffffff",
  baseColor: "#1f1f1f",
  sigColor: "#f5e0dc",
  animationSpeed: 5,
  sigSpeed: 15,
  sigLife: NaN,
  dirs: DIRS,
  axialWidth: 20,
  axialHeight: 8,
  gridRadius: 7,
  radius: 30,
  paused: false,
  deleteOnCollision: true,
  deleteOnPropagation: false,
  radial: true,
};

function isColor(strColor: string) {
  const s = new Option().style;
  s.color = strColor;
  return s.color !== "";
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
  return cubeToAxial({ x: qf, y: -qf - rf, z: rf });
}


function cubeToAxial(frac: Cube): Axial {
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
  // return { x: rx, y: ry, z: rz };
  return { q: rx, r: rz }
}

function updateConfig(): boolean {
  // Wow javascript is truly horrific
  let reset = false;

  const baseColor = (document.getElementById("baseColor") as HTMLInputElement).value;
  if (isColor(baseColor)) {
    CONFIG.baseColor = baseColor;
  }
  const sigColor = (document.getElementById("sigColor") as HTMLInputElement).value;
  if (isColor(sigColor)) {
    CONFIG.sigColor = sigColor;
  }
  const sigSpeed = parseInt((document.getElementById("sigSpeed") as HTMLInputElement).value, 10,);
  CONFIG.sigSpeed = sigSpeed;

  const sigLife = parseInt((document.getElementById("sigLife") as HTMLInputElement).value, 10,);
  CONFIG.sigLife = sigLife;

  const gridType = (document.getElementById("gridType") as HTMLInputElement).value;
  const radial = gridType === "radial";

  (document.getElementById("axialControls") as HTMLDivElement).style.display = radial ? "none" : "block";
  (document.getElementById("radialControls") as HTMLDivElement).style.display = radial ? "block" : "none";
  if (radial !== CONFIG.radial) {
    reset = true;
    CONFIG.radial = radial;
  }

  const axialWidth = parseInt((document.getElementById("axialWidth") as HTMLInputElement).value, 10);
  if (axialWidth !== CONFIG.axialWidth) {
    reset = true;
    CONFIG.axialWidth = axialWidth;
  }
  const axialHeight = parseInt((document.getElementById("axialHeight") as HTMLInputElement).value, 10);
  if (axialHeight !== CONFIG.axialHeight) {
    reset = true;
    CONFIG.axialHeight = axialHeight;
  }

  const gridRadius = parseInt((document.getElementById("gridRadius") as HTMLInputElement).value, 10);
  if (gridRadius !== CONFIG.gridRadius) {
    reset = true;
    CONFIG.gridRadius = gridRadius;
  }

  const paused = (document.getElementById("paused") as HTMLInputElement).checked;
  CONFIG.paused = paused;

  const deleteOnPropagation = (document.getElementById("deleteOnPropagation") as HTMLInputElement).checked;
  CONFIG.deleteOnPropagation = deleteOnPropagation;

  const deleteOnCollision = (document.getElementById("deleteOnCollision") as HTMLInputElement).checked;
  CONFIG.deleteOnCollision = deleteOnCollision;

  const dirs: Axial[] = Array.from(document.querySelectorAll<HTMLInputElement>(".dir")).filter((chk) => chk.checked).map((chk) => ({
    q: parseInt(chk.dataset.q || "0", 10),
    r: parseInt(chk.dataset.r || "0", 10),
  }));
  CONFIG.dirs = dirs;

  return reset;
}

class Signal {
  steps: number;
  color: string;
  travelSpeed: number;
  dirs: Axial[];
  lifespan: number;

  constructor(travelSpeed?: number, specialColor?: string) {
    this.steps = 0;
    this.color = specialColor ? specialColor : CONFIG.sigColor;
    this.dirs = CONFIG.dirs;
    this.travelSpeed = travelSpeed ? travelSpeed : CONFIG.sigSpeed;
    this.lifespan = CONFIG.sigLife;
  }

  get propagate(): Signal {
    return Object.assign(Object.create(Object.getPrototypeOf(this)), this);
  }

  get onCollision(): Signal | null {
    return null;
    // return propagate();
    // Or a secret third thing
  }

  iterate() {
    this.steps++;
  }
}

class Hexagon {
  axial: Axial;
  radius: number;
  originalColor: string;
  color: string;
  signal: Signal | null;
  path: Path2D;

  constructor(axial: Axial, radius: number, color: string) {
    this.axial = axial;
    this.radius = radius;
    this.originalColor = color;
    this.color = color;
    this.path = this.calculatePath();
  }

  get center(): Coord {
    return axialToPixel(this.axial, this.radius);
  }

  get vertices(): Coord[] {
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

  calculatePath(): Path2D {
    const path = new Path2D();
    const verts = this.vertices;
    path.moveTo(verts[0][0], verts[0][1]);
    for (let i = 1; i < 6; i++) {
      path.lineTo(verts[i][0], verts[i][1]);
    }
    path.closePath();
    return path;
  }

  updatePath() {
    this.path = this.calculatePath();
  }

  drawHexagon(
    ctx: CanvasRenderingContext2D,
    canvas: HTMLCanvasElement,
    fill: string,
  ) {
    const oW = canvas.width / 2;
    const oH = canvas.height / 2;
    ctx.save();
    ctx.translate(oW, oH);
    ctx.fillStyle = fill;
    ctx.fill(this.path);
    ctx.restore();
  }

  passSignal() {
    if (!this.signal) return;

    for (const direction of this.signal.dirs) {
      const newTargetKey = `${this.axial.q + direction.q},${this.axial.r + direction.r}`;
      const newTarget = HEXINDEX.get(newTargetKey);
      if (!newTarget) {
        continue;
      }
      if (!newTarget.signal) {
        newTarget.signal = this.signal?.propagate;
      } else {
        if (CONFIG.deleteOnCollision) {
          newTarget.signal = null;
        } else {
          newTarget.signal = this.signal?.propagate;
        }
      }
    }
    if (CONFIG.deleteOnPropagation) {
      this.signal = null;
    }
  }

  iterateAndDraw(
    ctx: CanvasRenderingContext2D,
    canvas: HTMLCanvasElement,
    fill: string,
  ) {
    this.signal?.iterate();
    if (this.signal?.steps / CONFIG.sigSpeed > CONFIG.sigLife) {
      this.signal = null;
    } else if (this.signal?.steps % CONFIG.sigSpeed === 0) {
      this.passSignal();
    }
    this.color = this.signal?.color ?? this.originalColor;

    this.drawHexagon(ctx, canvas, fill);
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

export function hexagonAxialFilled(radius: number): Axial[] {
  const out: Axial[] = [];
  for (let q = -radius; q <= radius; q++) {
    const rMin = Math.max(-radius, -q - radius);
    const rMax = Math.min(+radius, -q + radius);
    for (let r = rMin; r <= rMax; r++) {
      out.push({ q, r });
    }
  }
  return out;
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

  let axialField: Axial[];
  let hexArray: Hexagon[];

  function init() {
    hexArray = [];
    axialField = [];
    let hexagonRadiiAcross = 0;
    let hexagonRadiiDown = 0;
    let radius = CONFIG.radius;
    if (CONFIG.radial === true) {
      axialField = hexagonAxialFilled(CONFIG.gridRadius);
      hexagonRadiiAcross = CONFIG.gridRadius * (Math.sqrt(3) / 2 + 3);
      hexagonRadiiDown = CONFIG.gridRadius * (Math.sqrt(3) / 2 + 3);

      if (Math.max(hexagonRadiiAcross, hexagonRadiiDown) * radius > Math.min(canvas.width, canvas.height)) {
        radius = Math.min(canvas.width, canvas.height) / Math.max(hexagonRadiiAcross, hexagonRadiiDown);
      }
    } else {
      axialField = rectangleAxial(CONFIG.axialWidth, CONFIG.axialHeight);
      hexagonRadiiAcross = CONFIG.axialWidth * (2.5 - Math.sqrt(3) / 2)
      hexagonRadiiDown = CONFIG.axialHeight * (2.5 - Math.sqrt(3) / 2)

      if (Math.max(hexagonRadiiAcross, hexagonRadiiDown) * radius > Math.min(canvas.width, canvas.height)) {
        radius = Math.min(canvas.width, canvas.height) / Math.max(hexagonRadiiAcross, hexagonRadiiDown);
      }
    }

    CONFIG.radius = radius;
    hexArray = axialField.map(
      (a) => new Hexagon(a, CONFIG.radius, CONFIG.baseColor),
    );

    hexArray.forEach(h => h.updatePath());
    outlinePath = null;
  }
  init();
  for (const h of hexArray) HEXINDEX.set(`${h.axial.q},${h.axial.r}`, h);

  let hoveredKey: string | null = null;

  const hoverHandler = (e) => {
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left - canvas.width / 2;
    const my = e.clientY - rect.top - canvas.height / 2;
    const axial = pixelToAxial([mx, my], CONFIG.radius);
    const k = `${axial.q},${axial.r}`;
    hoveredKey = HEXINDEX.has(k) ? k : null;
  };

  const clickHandler = (e) => {
    console.log("clickt");
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left - canvas.width / 2;
    const my = e.clientY - rect.top - canvas.height / 2;
    const axial = pixelToAxial([mx, my], CONFIG.radius);
    const hexKey = `${axial.q},${axial.r}`;
    if (HEXINDEX.has(hexKey)) {
      const targetHex = HEXINDEX.get(hexKey);
      targetHex.signal = new Signal();
      targetHex.drawHexagon(ctx, canvas, targetHex.signal.color);
    }
  };

  canvas.addEventListener("mousemove", hoverHandler);
  canvas.addEventListener("click", clickHandler);

  async function render() {
    while (!RESET) {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      // ctx.fillStyle = CONFIG.bgColor;
      // ctx.fillRect(0, 0, canvas.width, canvas.height);
      for (const h of hexArray) {
        const hexKey = `${h.axial.q},${h.axial.r}`;
        const isHover = hoveredKey && hoveredKey === hexKey;
        const fill = isHover ? "red" : h.color;
        h.iterateAndDraw(ctx, canvas, fill);
      }

      drawOutlines(ctx, canvas, hexArray);

      await sleep(CONFIG.animationSpeed);
      while (CONFIG.paused) {
        await sleep(CONFIG.animationSpeed);
      }
    }
    canvas.removeEventListener("click", clickHandler);
    canvas.removeEventListener("mousemove", hoverHandler);
    RESET = false;
    return;
  }

  await render();
}

window.onload = async function () {
  // this should also include select
  const inputs = document.querySelectorAll<HTMLInputElement>(
    "#config input, #config select",
  );
  inputs.forEach((input) => {
    input.addEventListener("change", () => {
      const resetSignal = updateConfig();
      console.log(resetSignal);
      console.log("Updated CONFIG:", CONFIG);
      if (resetSignal) {
        RESET = true;
      }
    });
  });

  document.getElementById("reset").addEventListener("click", () => { RESET = true; });

  while (true) {
    console.log("Running hexagons:", CONFIG);
    await hexagons();
  }
};
