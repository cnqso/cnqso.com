// Crochet theme: the homepage as a hand-knit intarsia sweater. A harvest scene and the
// menu are charted onto a stitch grid (one cell per stitch, one yarn per cell), then
// every stitch is drawn as a plied, fuzzy V of yarn. Static: it redraws only on resize,
// and when the active row changes only the stitches around those rows.
// Loaded on demand by home-themes.ts.

const YARN: Record<string, string> = {
  cream: "#efe3c8",
  oatmeal: "#d8c49c",
  wheat: "#c9a565",
  gold: "#e3b84a",
  mustard: "#c99422",
  pumpkin: "#e2772a",
  orange: "#c75d1d",
  rust: "#93391c",
  burgundy: "#6b1f25",
  brown: "#5c3b22",
  bark: "#3a271b",
  olive: "#6e6b2b",
  sage: "#97a06d",
  forest: "#33502e",
  sky: "#a7bcc6",
  dusk: "#7c94a3",
  denim: "#4f6378",
  ink: "#1f1b19",
};
const NAMES = Object.keys(YARN);
const Y: Record<string, number> = {}; // yarn name -> palette index
NAMES.forEach((name, i) => (Y[name] = i));
const STITCHES_PER_ROW = 9; // a 7-stitch letter and a stitch above and below
const VARIANTS = 3; // sprite variations per yarn, so the fabric never looks tiled

// Small seeded random, so every redraw lays the same fibers in the same places.
function seeded(seed: number) {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// Knitting-chart letters, 5 stitches wide and 7 tall, one hex byte per row (top to
// bottom, high bit on the left): the classic LCD alphabet sweater lettering is built on.
const GLYPHS: Record<string, string> = {
  A: "0E1111111F1111", B: "1E11111E11111E", C: "0E11101010110E", D: "1C12111111121C",
  E: "1F10101E10101F", F: "1F10101E101010", G: "0E11101711110F",
  H: "1111111F111111", I: "0E04040404040E", J: "0702020202120C", K: "11121418141211",
  L: "1010101010101F", M: "111B1515111111", N: "11111915131111", O: "0E11111111110E",
  P: "1E11111E101010", Q: "0E11111115120D", R: "1E11111E141211", S: "0F10100E01011E",
  T: "1F040404040404", U: "1111111111110E", V: "11111111110A04", W: "1111111515150A",
  X: "11110A040A1111", Y: "1111110A040404", Z: "1F01020408101F",
  a: "00000E010F110F", b: "1010161911111E", c: "00000E1010110E", d: "01010D1311110F",
  e: "00000E111F100E", f: "0609081C080808", g: "000F11110F010E", h: "10101619111111",
  i: "04000C0404040E", j: "0200060202120C", k: "10101214181412", l: "0C04040404040E",
  m: "00001A15151111", n: "00001619111111", o: "00000E1111110E", p: "00001E111E1010",
  q: "00000D130F0101", r: "00001619101010", s: "00000E100E011E", t: "08081C08080906",
  u: "0000111111130D", v: "00001111110A04", w: "0000111115150A", x: "0000110A040A11",
  y: "000011110F010E", z: "00001F0204081F",
  "0": "0E11131519110E", "1": "040C040404040E", "2": "0E11010204081F",
  "3": "1F02040201110E", "4": "02060A121F0202", "5": "1F101E0101110E",
  "6": "0608101E11110E", "7": "1F010204080808", "8": "0E11110E11110E", "9": "0E11110F01020C",
  ".": "00000000000C0C", "/": "00010204081000", "-": "0000001F000000", "_": "0000000000001F",
};
const UNKNOWN = "1F11111111111F"; // a hollow box for anything not charted

function glyphRows(ch: string) {
  const hex = GLYPHS[ch] ?? UNKNOWN;
  return Array.from({ length: 7 }, (_, i) => parseInt(hex.slice(i * 2, i * 2 + 2), 16));
}

// Mix a yarn toward white (amount > 0) or black (amount < 0).
function shade(hex: string, amount: number, alpha = 1) {
  const n = parseInt(hex.slice(1), 16);
  const mix = (v: number) => Math.round(amount > 0 ? v + (255 - v) * amount : v * (1 + amount));
  return `rgba(${mix(n >> 16)},${mix((n >> 8) & 255)},${mix(n & 255)},${alpha})`;
}

// ---- The chart: a harvest scene painted stitch by stitch ----

function paintScene(cols: number, rows: number) {
  const chart = new Uint8Array(cols * rows);
  const s = rows / 100; // scene units: 100 stitches tall
  const set = (c: number, r: number, yarn: number) => {
    c = Math.round(c);
    r = Math.round(r);
    if (c >= 0 && c < cols && r >= 0 && r < rows) chart[r * cols + c] = yarn;
  };
  const rect = (c0: number, r0: number, c1: number, r1: number, yarn: number | ((c: number, r: number) => number)) => {
    for (let r = Math.round(r0); r <= Math.round(r1); r++)
      for (let c = Math.round(c0); c <= Math.round(c1); c++) set(c, r, typeof yarn === "number" ? yarn : yarn(c, r));
  };
  const line = (c0: number, r0: number, c1: number, r1: number, yarn: number) => {
    const steps = Math.max(Math.abs(c1 - c0), Math.abs(r1 - r0), 1);
    for (let i = 0; i <= steps; i++) set(c0 + ((c1 - c0) * i) / steps, r0 + ((r1 - r0) * i) / steps, yarn);
  };
  const oval = (cx: number, cy: number, rx: number, ry: number, yarn: (dx: number, dy: number) => number) => {
    for (let r = Math.floor(cy - ry); r <= Math.ceil(cy + ry); r++)
      for (let c = Math.floor(cx - rx); c <= Math.ceil(cx + rx); c++) {
        const dx = (c - cx) / rx;
        const dy = (r - cy) / ry;
        if (dx * dx + dy * dy <= 1) set(c, r, yarn(dx, dy));
      }
  };

  // Sky in knitted bands, each change of yarn zigzagged the way a knitter would.
  const bands: [number, number][] = [
    [0.2, Y.dusk],
    [0.38, Y.sky],
    [0.5, Y.oatmeal],
    [1, Y.wheat],
  ];
  for (let r = 0; r < rows; r++)
    for (let c = 0; c < cols; c++) {
      const zig = (Math.abs((c % 6) - 3) - 1.5) * 0.6;
      const v = (r + zig) / rows;
      set(c, r, bands.find(([edge]) => v < edge)![1]);
    }

  // A low harvest sun and a few crows.
  oval(cols * 0.8, rows * 0.36, 9 * s, 9 * s, (dx, dy) => (dx * dx + dy * dy > 0.72 ? Y.mustard : Y.gold));
  for (const [u, v] of [
    [0.3, 0.14],
    [0.36, 0.1],
    [0.41, 0.17],
    [0.62, 0.2],
  ]) {
    const c = u * cols;
    const r = v * rows;
    line(c - 2 * s, r - 1 * s, c, r, Y.ink);
    line(c, r, c + 2 * s, r - 1 * s, Y.ink);
  }

  // Rolling hills, far then near.
  for (let c = 0; c < cols; c++) {
    const u = c / cols;
    rect(c, rows * (0.5 + 0.03 * Math.sin(u * 7) + 0.015 * Math.sin(u * 17)), c, rows, Y.sage);
    rect(c, rows * (0.56 + 0.03 * Math.sin(u * 5 + 1) + 0.01 * Math.sin(u * 23)), c, rows, Y.olive);
  }

  // Ploughed field: brown with darker furrows.
  const field = Math.round(rows * 0.63);
  rect(0, field, cols - 1, rows - 1, (c, r) => ((r - field) % 4 === 3 ? Y.bark : Y.brown));

  // Dried corn: a back row and a taller front row, stalks with leaves, cobs and tassels.
  const corn = (base: number, height: number, spacing: number, offset: number) => {
    for (let c = offset; c < cols; c += spacing) {
      const top = base - height;
      line(c, base, c, top, Y.wheat);
      for (let k = 1; k <= 4; k++) {
        const r = base - (height * k) / 5;
        const side = k % 2 ? 1 : -1;
        line(c, r, c + side * 3 * s, r - 2.2 * s, k % 3 ? Y.olive : Y.wheat);
      }
      const cob = base - height * 0.55;
      rect(c + 1, cob - 2 * s, c + 1 + s, cob + 1.5 * s, (cc, rr) => (rr % 2 ? Y.gold : Y.mustard));
      set(c, top - 1, Y.gold);
      set(c - 1, top, Y.gold);
      set(c + 1, top, Y.gold);
    }
  };
  corn(rows * 0.66, 13 * s, Math.max(3, Math.round(4 * s)), 1);
  corn(rows * 0.8, 22 * s, Math.max(5, Math.round(7 * s)), 3);

  // Scarecrows: post and crossbar, plaid shirt, denim legs, burlap head, straw hat.
  const scarecrow = (u: number, base: number, size: number) => {
    const cx = Math.round(cols * u);
    const h = 34 * size;
    const arms = base - h * 0.7;
    line(cx, base, cx, base - h, Y.bark);
    line(cx - 9 * size, arms, cx + 9 * size, arms, Y.bark);
    const plaid = (c: number, r: number) =>
      c % 3 === 0 ? Y.mustard : (Math.floor(c / 2) + Math.floor(r / 2)) % 2 ? Y.rust : Y.burgundy;
    rect(cx - 4 * size, arms - 1, cx + 4 * size, base - h * 0.38, plaid);
    rect(cx - 9 * size, arms - 1, cx + 9 * size, arms + 1.5 * size, plaid);
    for (const side of [-1, 1]) {
      const hand = cx + side * 9.5 * size;
      line(hand, arms - 1, hand + side * 2 * size, arms - 2 * size, Y.gold);
      line(hand, arms + 1, hand + side * 2 * size, arms + 2 * size, Y.gold);
      rect(cx + side * 1.5 * size - size, base - h * 0.38, cx + side * 1.5 * size + size, base - h * 0.12, Y.denim);
    }
    const head = base - h * 0.84;
    oval(cx, head, 3.3 * size, 3.6 * size, () => Y.oatmeal);
    set(cx - 1.3 * size, head - 0.5 * size, Y.ink);
    set(cx + 1.3 * size, head - 0.5 * size, Y.ink);
    line(cx - 1.6 * size, head + 1.6 * size, cx + 1.6 * size, head + 1.6 * size, Y.rust);
    rect(cx - 6 * size, head - 3.6 * size, cx + 6 * size, head - 3.2 * size, Y.mustard);
    rect(cx - 3 * size, head - 7 * size, cx + 3 * size, head - 3.8 * size, Y.wheat);
    rect(cx - 3 * size, head - 4.6 * size, cx + 3 * size, head - 4 * size, Y.rust);
  };
  scarecrow(0.2, rows * 0.84, s);
  scarecrow(0.66, rows * 0.79, 0.8 * s);

  // A hay bale and a patch of pumpkins in front.
  const hay = cols * 0.42;
  rect(hay, rows * 0.84, hay + 14 * s, rows * 0.92, (c, r) =>
    Math.round(c - hay) % Math.round(5 * s) === 0 ? Y.bark : r % 2 ? Y.wheat : Y.mustard,
  );
  const pumpkin = (u: number, v: number, size: number) => {
    const cx = cols * u;
    const cy = rows * v;
    oval(cx, cy, 7 * size, 4.6 * size, (dx, dy) => {
      if (dy > 0.55) return Y.rust;
      const rib = Math.abs(Math.sin(dx * Math.PI * 1.5));
      return rib < 0.18 ? Y.orange : dx < -0.2 && dy < -0.1 ? Y.pumpkin : Y.pumpkin;
    });
    rect(cx - 0.5 * size, cy - 6 * size, cx + 0.5 * size, cy - 4.4 * size, Y.forest);
    line(cx + size, cy - 5 * size, cx + 4 * size, cy - 6.5 * size, Y.forest);
  };
  pumpkin(0.08, 0.93, s);
  pumpkin(0.33, 0.95, 0.75 * s);
  pumpkin(0.56, 0.94, 1.15 * s);
  pumpkin(0.8, 0.91, 0.85 * s);
  pumpkin(0.93, 0.96, 1.05 * s);

  // A fair-isle border top and bottom: the aunt's signature.
  const border = (r0: number) => {
    for (let c = 0; c < cols; c++) {
      set(c, r0, Y.burgundy);
      set(c, r0 + 1, c % 4 === 0 ? Y.rust : Y.cream);
      set(c, r0 + 2, Math.abs((c % 6) - 3) === 1 ? Y.burgundy : Y.cream);
      set(c, r0 + 3, c % 4 === 2 ? Y.rust : Y.cream);
      set(c, r0 + 4, Y.burgundy);
    }
  };
  border(0);
  border(rows - 5);
  return chart;
}

// ---- One stitch: a knit V of two plied, fuzzy legs ----

function drawStitch(g: CanvasRenderingContext2D, hex: string, sw: number, sh: number, rand: () => number) {
  for (const side of [-1, 1]) {
    const rx = sw * 0.26;
    const ry = sh * 0.72;
    g.save();
    g.translate(sw * (0.5 + side * 0.22), sh * 0.4);
    g.rotate(side * 0.4);

    // Body: lit from the upper left, deep in the crease between legs.
    const body = g.createRadialGradient(-rx * 0.35, -ry * 0.35, 0, 0, 0, ry * 1.05);
    body.addColorStop(0, shade(hex, 0.22));
    body.addColorStop(0.5, hex);
    body.addColorStop(1, shade(hex, -0.45));
    g.beginPath();
    g.ellipse(0, 0, rx, ry, 0, 0, Math.PI * 2);
    g.fillStyle = body;
    g.fill();

    // Ply: the twist of the strands, a dark and a light line on the diagonal.
    g.save();
    g.clip();
    g.lineWidth = Math.max(0.4, sw * 0.05);
    for (let k = -5; k <= 5; k++) {
      const y = k * ry * 0.22 + (rand() - 0.5) * ry * 0.06;
      g.strokeStyle = shade(hex, -0.35, 0.4);
      g.beginPath();
      g.moveTo(-rx, y - rx * 0.9);
      g.lineTo(rx, y + rx * 0.9);
      g.stroke();
      g.strokeStyle = shade(hex, 0.3, 0.22);
      g.beginPath();
      g.moveTo(-rx, y - rx * 0.9 + sw * 0.07);
      g.lineTo(rx, y + rx * 0.9 + sw * 0.07);
      g.stroke();
    }
    g.restore();

    // Fuzz: a halo of fine fibers escaping the strand, and the odd long stray.
    for (let n = 0; n < 34; n++) {
      const stray = n < 3;
      const a = rand() * Math.PI * 2;
      const x = Math.cos(a) * rx * 0.92;
      const y = Math.sin(a) * ry * 0.92;
      const nx = Math.cos(a) / rx;
      const ny = Math.sin(a) / ry;
      const norm = Math.hypot(nx, ny);
      const length = sw * (stray ? 0.45 + rand() * 0.45 : 0.06 + rand() * 0.2);
      const bend = (rand() - 0.5) * length;
      g.strokeStyle = shade(hex, 0.15 + rand() * 0.25, stray ? 0.32 : 0.25 + rand() * 0.25);
      g.lineWidth = 0.3 + rand() * 0.25;
      g.beginPath();
      g.moveTo(x, y);
      g.quadraticCurveTo(
        x + (nx / norm) * length * 0.5 - (ny / norm) * bend,
        y + (ny / norm) * length * 0.5 + (nx / norm) * bend,
        x + (nx / norm) * length,
        y + (ny / norm) * length,
      );
      g.stroke();
    }
    g.restore();
  }
}

export async function createCrochet(canvas: HTMLCanvasElement, rows: HTMLElement[]) {
  await document.fonts.load('1em "Archivo Black"');
  const ctx = canvas.getContext("2d")!;
  const dpr = Math.min(window.devicePixelRatio, 2);

  let width = 0;
  let height = 0;
  let sw = 10; // stitch size in CSS px
  let sh = 9;
  let cols = 0;
  let lines = 0;
  let padX = 0;
  let padY = 0;
  let sprites: HTMLCanvasElement[][] = [];
  let chart = new Uint8Array(0);
  let letter = new Int8Array(0); // which menu row knitted this stitch as a letter, or -1
  let outline = new Uint8Array(0);
  let band = new Int8Array(0); // which menu row each stitch row sits in
  let fibers: { x: number; y: number; cx: number; cy: number; ex: number; ey: number; w: number }[] = [];
  let active = 0;
  let running = false;

  function makeSprites() {
    padX = sw * 0.35;
    padY = sh * 0.6;
    sprites = NAMES.map((name, i) =>
      Array.from({ length: VARIANTS }, (_, v) => {
        const sprite = document.createElement("canvas");
        sprite.width = Math.ceil((sw + padX * 2) * dpr);
        sprite.height = Math.ceil((sh + padY * 2) * dpr);
        const g = sprite.getContext("2d")!;
        g.scale(dpr, dpr);
        g.translate(padX, padY);
        drawStitch(g, YARN[name], sw, sh, seeded(i * 7919 + v * 104729 + 1));
        return sprite;
      }),
    );
  }

  // Knit the menu from the chart alphabet: each letter centred on its (invisible) HTML
  // letter, scaled up by whole stitches when the row has room.
  function chartLetters() {
    letter = new Int8Array(cols * lines).fill(-1);
    rows.forEach((row, index) => {
      const box = row.getBoundingClientRect();
      const spans = (Array.prototype.slice.call(row.children) as HTMLElement[]).filter((span) => (span.textContent || "").trim());
      const room = Math.min((box.height / sh - 2) / 7, (width / spans.length / sw - 1) / 5);
      const k = Math.max(1, Math.floor(room));
      const top = Math.round(box.top / sh + (box.height / sh - 7 * k) / 2);
      for (const span of spans) {
        const middle = span.getBoundingClientRect();
        const left = Math.round((middle.left + middle.width / 2) / sw - (5 * k) / 2);
        glyphRows(span.textContent || "").forEach((bits, y) => {
          for (let x = 0; x < 5; x++) {
            if (!(bits & (16 >> x))) continue;
            for (let dy = 0; dy < k; dy++)
              for (let dx = 0; dx < k; dx++) {
                const c = left + x * k + dx;
                const r = top + y * k + dy;
                if (c >= 0 && c < cols && r >= 0 && r < lines) letter[r * cols + c] = index;
              }
          }
        });
      }
    });
    outline = new Uint8Array(cols * lines);
    for (let r = 0; r < lines; r++)
      for (let c = 0; c < cols; c++) {
        if (letter[r * cols + c] >= 0) continue;
        for (let dr = -1; dr <= 1 && !outline[r * cols + c]; dr++)
          for (let dc = -1; dc <= 1; dc++) {
            const rr = r + dr;
            const cc = c + dc;
            if (rr >= 0 && rr < lines && cc >= 0 && cc < cols && letter[rr * cols + cc] >= 0) outline[r * cols + c] = 1;
          }
      }
    band = new Int8Array(lines);
    const tops = rows.map((row) => row.getBoundingClientRect());
    for (let r = 0; r < lines; r++) {
      const y = (r + 0.5) * sh;
      band[r] = tops.findIndex((box) => y >= box.top && y < box.bottom);
    }
  }

  // The yarn for one stitch: letters in cream with a dark outline over the scene; the
  // active row knitted as a cream band with rust letters.
  function yarnAt(i: number, r: number) {
    const own = letter[i];
    if (own >= 0) return own === active ? Y.rust : Y.cream;
    if (band[r] === active) return Y.cream;
    return outline[i] ? Y.bark : chart[i];
  }

  function layout() {
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
    const rowHeight = height / rows.length;
    const longest = Math.max(...rows.map((row) => row.children.length));
    sh = Math.min(12, rowHeight / STITCHES_PER_ROW, width / (longest * 6.5) / 1.18);
    sw = sh * 1.18;
    cols = Math.ceil(width / sw);
    lines = Math.ceil(height / sh);
    makeSprites();
    chart = paintScene(cols, lines);
    chartLetters();
    const rand = seeded(cols * 31 + lines);
    fibers = Array.from({ length: Math.round(cols * lines * 0.12) }, () => {
      const x = rand() * width;
      const y = rand() * height;
      const a = rand() * Math.PI * 2;
      const length = sw * (0.6 + rand() * 1.6);
      return {
        x,
        y,
        cx: x + Math.cos(a + 0.9) * length * 0.6,
        cy: y + Math.sin(a + 0.9) * length * 0.6,
        ex: x + Math.cos(a) * length,
        ey: y + Math.sin(a) * length,
        w: 0.3 + rand() * 0.35,
      };
    });
  }

  // Paint the stitches whose rows touch [top, bottom] (CSS px), from scratch.
  function draw(top: number, bottom: number) {
    ctx.save();
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.beginPath();
    ctx.rect(0, top, width, bottom - top);
    ctx.clip();
    ctx.fillStyle = "#1e130d";
    ctx.fillRect(0, top, width, bottom - top);

    const first = Math.max(0, Math.floor(top / sh) - 2);
    const last = Math.min(lines - 1, Math.ceil(bottom / sh) + 2);
    // Deep shadow under each stitch, then the stitches bottom row first, so each row's
    // legs tuck beneath the row above as in real knitting.
    for (let r = first; r <= last; r++)
      for (let c = 0; c < cols; c++) {
        ctx.fillStyle = shade(YARN[NAMES[yarnAt(r * cols + c, r)]], -0.62);
        ctx.fillRect(c * sw, r * sh, sw + 0.5, sh + 0.5);
      }
    const w = sprites[0][0].width / dpr;
    const h = sprites[0][0].height / dpr;
    for (let r = last; r >= first; r--)
      for (let c = 0; c < cols; c++) {
        const variant = (((c * 73856093) ^ (r * 19349663)) >>> 0) % VARIANTS;
        ctx.drawImage(sprites[yarnAt(r * cols + c, r)][variant], c * sw - padX, r * sh - padY, w, h);
      }

    // Stray fibers lying across the fabric, each in the colour of the yarn it left.
    for (const f of fibers) {
      if (f.y < top - 30 || f.y > bottom + 30) continue;
      const c = Math.min(cols - 1, Math.floor(f.x / sw));
      const r = Math.min(lines - 1, Math.floor(f.y / sh));
      ctx.strokeStyle = shade(YARN[NAMES[yarnAt(r * cols + c, r)]], 0.3, 0.3);
      ctx.lineWidth = f.w;
      ctx.beginPath();
      ctx.moveTo(f.x, f.y);
      ctx.quadraticCurveTo(f.cx, f.cy, f.ex, f.ey);
      ctx.stroke();
    }

    // Drape: soft folds of light and shadow, then a warm vignette.
    ctx.globalCompositeOperation = "soft-light";
    const folds = ctx.createLinearGradient(0, 0, width, height * 0.6);
    [0, 0.18, 0.37, 0.55, 0.74, 1].forEach((stop, k) =>
      folds.addColorStop(stop, k % 2 ? "rgba(0,0,0,0.28)" : "rgba(255,240,220,0.22)"),
    );
    ctx.fillStyle = folds;
    ctx.fillRect(0, top, width, bottom - top);
    ctx.globalCompositeOperation = "multiply";
    const vignette = ctx.createRadialGradient(width / 2, height / 2, Math.min(width, height) * 0.35, width / 2, height / 2, Math.max(width, height) * 0.75);
    vignette.addColorStop(0, "rgba(255,255,255,1)");
    vignette.addColorStop(1, "rgba(150,110,80,1)");
    ctx.fillStyle = vignette;
    ctx.fillRect(0, top, width, bottom - top);
    ctx.restore();
  }

  function setActive(index: number) {
    if (index === active || !running) return;
    const rowsToRedraw = [rows[active], rows[index]].map((row) => row.getBoundingClientRect());
    active = index;
    for (const box of rowsToRedraw) draw(box.top - sh * 2, box.bottom + sh * 3);
  }

  let hovered: number | null = null;
  rows.forEach((row, index) => {
    row.addEventListener("mouseenter", () => {
      hovered = index;
      setActive(index);
    });
    row.addEventListener("mouseleave", () => {
      if (hovered !== index) return;
      hovered = null;
      setActive(0);
    });
  });

  let pending = 0;
  window.addEventListener("resize", () => {
    if (!running) return;
    cancelAnimationFrame(pending);
    pending = requestAnimationFrame(() => {
      layout();
      draw(0, height);
    });
  });

  return {
    start() {
      if (running) return;
      running = true;
      active = hovered ?? 0;
      layout();
      draw(0, height);
      document.documentElement.classList.add("crochet-ready");
    },
    stop() {
      running = false;
      document.documentElement.classList.remove("crochet-ready");
    },
  };
}
