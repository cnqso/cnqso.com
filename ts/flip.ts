// Flip theme: the homepage as the main menu of a mid-2000s Arabic-language flip phone,
// as if photographed. A small screen in 4096 colours with ordered dithering is drawn up
// through red, green and blue subpixels, bloom and a slightly soft camera, under glass
// with fingerprints, smears, dust and a window's reflection. Legible or not, by now you
// know the top of the screen changes the theme. Loaded on demand by home-themes.ts.

const LABELS: Record<string, string> = {
  "CNQSO.COM": "كنقسو",
  SOKODLE: "سوكودل",
  "INFINITE SUGAR": "سكر لا نهائي",
  "/hexagons": "سداسيات",
  "/spirals": "حلزونات",
  "/bloonsbench": "بلونز بنش",
  "/esotericbench": "معيار باطني",
  GITHUB: "جيت هب",
  ODIR: "الدليل المفتوح",
  BLOG: "المدونة",
};
const DIGITS = "٠١٢٣٤٥٦٧٨٩";
const FONT = '"Geeza Pro", "Noto Naskh Arabic", "Noto Sans Arabic", "Segoe UI", Tahoma, sans-serif';
const BACKLIGHT_MS = 10000; // idle time before the screen dims

// 8x8 menu icons, one hex byte per row, by menu row (the title row has none).
const ICONS = [
  "",
  "FF81BDA5A5BD81FF", // a sokoban crate
  "00663C7E7E3C6600", // a wrapped sweet
  "1824424242422418", // a hexagon
  "3C4299A5A5B9827C", // a spiral
  "3C7E7E7E3C180810", // a balloon
  "1818FF7E3C664200", // a star
  "81C3FFBDFFE77E3C", // a cat
  "00708F818181FF00", // a folder
  "7C4647415D415D7F", // a page
];

// Screen pixels are 2-4 CSS pixels: chunky like the era, still legible on a desktop.
const pixelSize = () => Math.max(2, Math.min(4, Math.round(window.innerWidth / 360)));

// Small seeded random, so the smudges land in the same places on every redraw.
function seeded(seed: number) {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// Ordered dithering down to 4 bits per channel, the colour depth of the era.
const BAYER = [0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5];

export async function createFlip(canvas: HTMLCanvasElement, rows: HTMLElement[]) {
  const root = document.documentElement;
  // The phone's own framebuffer, small; the visible canvas is the "photo" of it.
  const screen = document.createElement("canvas");
  const ctx = screen.getContext("2d", { willReadFrequently: true })!;
  const photo = canvas.getContext("2d")!;
  const sharp = document.createElement("canvas").getContext("2d")!;
  // The glass is its own layer, so the smudges stay when the backlight dims.
  const glass = document.createElement("canvas");
  glass.className = "flip-glass";
  glass.setAttribute("aria-hidden", "true");
  canvas.after(glass);
  let tile = 4; // device pixels per screen pixel
  let subpixels: CanvasPattern | null = null;
  let grain: CanvasPattern | null = null;
  const ink = document.createElement("canvas").getContext("2d", { willReadFrequently: true })!;
  let px = 4; // CSS pixels per screen pixel
  let w = 0;
  let h = 0;
  let active = 0;
  let hovered: number | null = null;
  let running = false;
  let ticker = 0;
  let lastMove = performance.now();

  function layout() {
    px = pixelSize();
    w = Math.ceil(window.innerWidth / px);
    h = Math.ceil(window.innerHeight / px);
    screen.width = ink.canvas.width = w;
    screen.height = ink.canvas.height = h;
    tile = Math.max(2, Math.round(px * Math.min(window.devicePixelRatio, 2)));
    canvas.width = sharp.canvas.width = w * tile;
    canvas.height = sharp.canvas.height = h * tile;
    canvas.style.width = `${w * px}px`;
    canvas.style.height = `${h * px}px`;
    subpixels = photo.createPattern(subpixelTile(), "repeat");
    grain = photo.createPattern(grainTile(), "repeat");
    paintGlass();
  }

  // One screen pixel: red, green and blue stripes with a dark gap below and beside.
  function subpixelTile() {
    const t = document.createElement("canvas");
    t.width = t.height = tile;
    const g = t.getContext("2d")!;
    g.fillStyle = "#3a3a3a";
    g.fillRect(0, 0, tile, tile);
    const gap = Math.max(1, Math.round(tile * 0.16));
    const stripe = (tile - gap) / 3;
    ["#ffb3a6", "#b6ffb3", "#b3c6ff"].forEach((colour, i) => {
      g.fillStyle = colour;
      g.fillRect(i * stripe, 0, stripe, tile - gap);
    });
    return t;
  }

  function grainTile() {
    const t = document.createElement("canvas");
    t.width = t.height = 128;
    const g = t.getContext("2d")!;
    const noise = g.createImageData(128, 128);
    const rand = seeded(7);
    for (let i = 0; i < noise.data.length; i += 4) {
      const v = 128 + (rand() - 0.5) * 100;
      noise.data[i] = noise.data[i + 1] = noise.data[i + 2] = v;
      noise.data[i + 3] = 255;
    }
    g.putImageData(noise, 0, 0);
    return t;
  }

  // Fingerprints, swipe smears, dust, a milky film and a window reflected in the glass.
  function paintGlass() {
    const W = window.innerWidth;
    const H = window.innerHeight;
    const dpr = Math.min(window.devicePixelRatio, 2);
    glass.width = Math.round(W * dpr);
    glass.height = Math.round(H * dpr);
    const g = glass.getContext("2d")!;
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    const rand = seeded(Math.round(W) * 7 + Math.round(H));

    g.fillStyle = "rgba(255,255,255,0.05)";
    g.fillRect(0, 0, W, H);

    // A window behind the photographer: two soft panes, upper left.
    g.save();
    g.filter = `blur(${Math.round(Math.min(W, H) * 0.035)}px)`;
    g.translate(W * 0.12, -H * 0.04);
    g.rotate(-0.1);
    g.fillStyle = "rgba(225,238,255,0.14)";
    g.fillRect(0, 0, W * 0.15, H * 0.5);
    g.fillRect(W * 0.165, 0, W * 0.15, H * 0.5);
    g.restore();

    // A broad sheen across the glass.
    g.save();
    g.filter = `blur(${Math.round(Math.min(W, H) * 0.06)}px)`;
    const sheen = g.createLinearGradient(0, 0, W, H);
    sheen.addColorStop(0.3, "rgba(255,255,255,0)");
    sheen.addColorStop(0.45, "rgba(255,255,255,0.06)");
    sheen.addColorStop(0.6, "rgba(255,255,255,0)");
    g.fillStyle = sheen;
    g.fillRect(0, 0, W, H);
    g.restore();

    // Swipe smears where a thumb has wiped the screen.
    g.save();
    g.filter = `blur(${Math.round(Math.min(W, H) * 0.02)}px)`;
    g.lineCap = "round";
    for (let i = 0; i < 4; i++) {
      g.strokeStyle = `rgba(255,255,255,${0.05 + rand() * 0.05})`;
      g.lineWidth = Math.min(W, H) * (0.05 + rand() * 0.06);
      const x = W * (0.25 + rand() * 0.6);
      const y = H * (0.2 + rand() * 0.7);
      g.beginPath();
      g.arc(x, y, Math.min(W, H) * (0.2 + rand() * 0.3), -2.6 + rand(), -1.2 + rand());
      g.stroke();
    }
    g.restore();

    // Fingerprints: whorls of broken ridges, mostly where a thumb lands.
    g.save();
    g.filter = "blur(0.6px)";
    for (let p = 0; p < 4; p++) {
      const cx = W * (p === 0 ? 0.78 : 0.15 + rand() * 0.75);
      const cy = H * (p === 0 ? 0.72 : 0.15 + rand() * 0.75);
      const size = Math.min(W, H) * (0.06 + rand() * 0.04);
      const tilt = rand() * Math.PI;
      const alpha = 0.1 + rand() * 0.07;
      for (let k = 2; k < 26; k++) {
        const r = (size * k) / 26;
        g.strokeStyle = `rgba(255,255,255,${alpha * (1 - k / 34)})`;
        g.lineWidth = Math.max(0.8, size / 40);
        g.setLineDash([2 + rand() * 9, 1 + rand() * 4]);
        g.beginPath();
        for (let a = 0; a <= 64; a++) {
          const t = (a / 64) * Math.PI * 2;
          const wobble = 1 + 0.04 * Math.sin(t * 5 + k * 0.7);
          const x = Math.cos(t) * r * 0.72 * wobble;
          const y = Math.sin(t) * r * wobble;
          const rx = cx + x * Math.cos(tilt) - y * Math.sin(tilt);
          const ry = cy + x * Math.sin(tilt) + y * Math.cos(tilt);
          if (a === 0) g.moveTo(rx, ry);
          else g.lineTo(rx, ry);
        }
        g.stroke();
      }
    }
    g.setLineDash([]);
    g.restore();

    // Dust, and the odd fibre.
    for (let i = 0; i < 90; i++) {
      g.fillStyle = `rgba(255,255,255,${0.2 + rand() * 0.45})`;
      g.beginPath();
      g.arc(rand() * W, rand() * H, 0.4 + rand() * 1.1, 0, Math.PI * 2);
      g.fill();
    }
    g.strokeStyle = "rgba(255,255,255,0.25)";
    g.lineWidth = 0.6;
    for (let i = 0; i < 5; i++) {
      const x = rand() * W;
      const y = rand() * H;
      g.beginPath();
      g.moveTo(x, y);
      g.quadraticCurveTo(x + 10 * rand(), y + 14 * rand(), x + 22 * (rand() - 0.5), y + 22 * rand());
      g.stroke();
    }
  }

  // The "photo": framebuffer through subpixels and bloom, a slightly soft camera, an
  // unevenly lit backlight, film grain.
  function present() {
    const W = canvas.width;
    const H = canvas.height;
    const s = sharp;
    s.globalCompositeOperation = "source-over";
    s.imageSmoothingEnabled = false;
    s.drawImage(screen, 0, 0, W, H);
    s.globalCompositeOperation = "multiply";
    s.fillStyle = subpixels!;
    s.fillRect(0, 0, W, H);
    // Bright pixels bleed light into their neighbours.
    s.globalCompositeOperation = "screen";
    s.globalAlpha = 0.5;
    s.imageSmoothingEnabled = true;
    s.filter = `blur(${tile * 1.3}px)`;
    s.drawImage(screen, 0, 0, W, H);
    s.filter = "none";
    s.globalAlpha = 1;
    s.globalCompositeOperation = "source-over";

    photo.filter = `blur(${Math.max(0.7, tile * 0.2)}px) contrast(0.88) saturate(0.9) brightness(1.08)`;
    photo.drawImage(s.canvas, 0, 0);
    photo.filter = "none";
    photo.globalCompositeOperation = "multiply";
    const backlight = photo.createRadialGradient(W * 0.5, H * 0.58, Math.min(W, H) * 0.15, W * 0.5, H * 0.58, Math.max(W, H) * 0.8);
    backlight.addColorStop(0, "#ffffff");
    backlight.addColorStop(1, "#8a8c99");
    photo.fillStyle = backlight;
    photo.fillRect(0, 0, W, H);
    photo.globalCompositeOperation = "screen";
    const bleed = photo.createLinearGradient(0, H, 0, H * 0.8);
    bleed.addColorStop(0, "rgba(120,170,255,0.16)");
    bleed.addColorStop(1, "rgba(120,170,255,0)");
    photo.fillStyle = bleed;
    photo.fillRect(0, 0, W, H);
    photo.globalCompositeOperation = "overlay";
    photo.globalAlpha = 0.12;
    photo.fillStyle = grain!;
    photo.fillRect(0, 0, W, H);
    photo.globalAlpha = 1;
    photo.globalCompositeOperation = "source-over";
  }

  // A menu row's band on the screen, in screen pixels.
  function band(index: number) {
    const box = rows[index].getBoundingClientRect();
    return [Math.round(box.top / px), Math.round(box.bottom / px)];
  }

  // Text the way the phone drew it: shaped by the browser, then reduced to solid pixels
  // with a single half-tone at the edges.
  function text(str: string, x: number, baseline: number, size: number, color: string, align: CanvasTextAlign) {
    ink.clearRect(0, 0, w, h);
    ink.font = `bold ${size}px ${FONT}`;
    ink.direction = /[؀-ۿ]/.test(str) ? "rtl" : "ltr";
    ink.textAlign = align;
    ink.fillStyle = "#fff";
    ink.fillText(str, x, baseline);
    // Only the band the text can reach, not the whole screen.
    const top = Math.max(0, Math.floor(baseline - size * 1.2));
    const rows = Math.min(h, Math.ceil(baseline + size * 0.6)) - top;
    const mask = ink.getImageData(0, top, w, rows).data;
    ctx.fillStyle = color;
    for (let y = 0; y < rows; y++)
      for (let x = 0; x < w; x++) {
        const alpha = mask[(y * w + x) * 4 + 3];
        if (alpha < 60) continue;
        ctx.globalAlpha = alpha > 150 ? 1 : 0.5;
        ctx.fillRect(x, top + y, 1, 1);
      }
    ctx.globalAlpha = 1;
  }

  function icon(bits: string, right: number, top: number, scale: number, color: string) {
    ctx.fillStyle = color;
    for (let y = 0; y < 8; y++) {
      const row = parseInt(bits.slice(y * 2, y * 2 + 2), 16);
      for (let x = 0; x < 8; x++) if (row & (128 >> x)) ctx.fillRect(right - (8 - x) * scale, top + y * scale, scale, scale);
    }
  }

  function wallpaper() {
    const sky = ctx.createLinearGradient(0, 0, 0, h);
    sky.addColorStop(0, "#020d3a");
    sky.addColorStop(0.55, "#0a3f9e");
    sky.addColorStop(1, "#38a6f0");
    ctx.fillStyle = sky;
    ctx.fillRect(0, 0, w, h);
    // The stock swirl: glowing arcs and a few soft lights.
    ctx.lineWidth = Math.max(1, w / 90);
    for (const [cx, cy, r, alpha] of [
      [w * 1.1, h * 1.25, h * 0.95, 0.35],
      [w * 1.05, h * 1.3, h * 0.8, 0.22],
      [-w * 0.2, h * 1.4, h * 0.9, 0.18],
    ]) {
      ctx.strokeStyle = `rgba(150,220,255,${alpha})`;
      ctx.beginPath();
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
      ctx.stroke();
    }
    for (const [u, v, r] of [
      [0.2, 0.3, 0.05],
      [0.75, 0.55, 0.08],
      [0.45, 0.8, 0.06],
    ]) {
      const glow = ctx.createRadialGradient(u * w, v * h, 0, u * w, v * h, r * w);
      glow.addColorStop(0, "rgba(180,230,255,0.35)");
      glow.addColorStop(1, "rgba(180,230,255,0)");
      ctx.fillStyle = glow;
      ctx.fillRect(0, 0, w, h);
    }
  }

  function highlight(top: number, bottom: number) {
    const bar = ctx.createLinearGradient(0, top, 0, bottom);
    bar.addColorStop(0, "#ffd27a");
    bar.addColorStop(0.5, "#f2a531");
    bar.addColorStop(1, "#c86f0c");
    ctx.fillStyle = bar;
    ctx.fillRect(2, top, w - 4, bottom - top);
    ctx.fillStyle = "#fff3d6";
    ctx.fillRect(2, top, w - 4, 1);
  }

  function arabicNumber(n: number) {
    return String(n).replace(/\d/g, (d) => DIGITS[+d]).padStart(2, DIGITS[0]);
  }

  function draw() {
    wallpaper();

    // Status bar: signal, clock, the network's name, battery.
    const [top, bottom] = band(0);
    const header = ctx.createLinearGradient(0, top, 0, bottom);
    header.addColorStop(0, active === 0 ? "#ffd27a" : "#2b4f9e");
    header.addColorStop(1, active === 0 ? "#c86f0c" : "#071b52");
    ctx.fillStyle = header;
    ctx.fillRect(0, top, w, bottom - top);
    const tall = bottom - top;
    const fg = active === 0 ? "#1a0f00" : "#ffffff";
    const bars = 3 + Math.floor((Date.now() / 7000) % 3); // reception wanders
    const u = Math.max(1, Math.round(tall / 12)); // status icon unit
    ctx.fillStyle = fg;
    for (let b = 0; b < 5; b++) {
      const barHeight = Math.round(((b + 1) / 5) * tall * 0.55);
      if (b < bars) ctx.fillRect(3 + b * 3 * u, top + tall * 0.75 - barHeight, 2 * u, barHeight);
      else ctx.fillRect(3 + b * 3 * u, top + tall * 0.75 - u, 2 * u, u);
    }
    const now = new Date();
    const clock = `${arabicNumber(now.getHours() % 12 || 12)}:${arabicNumber(now.getMinutes())}`;
    text(clock, 6 + 15 * u, top + tall * 0.72, Math.round(tall * 0.55), fg, "left");
    text(LABELS[rows[0].dataset.text || ""] || "كنقسو", w / 2, top + tall * 0.72, Math.round(tall * 0.62), fg, "center");
    const battery = Math.round(top + tall * 0.32);
    ctx.fillRect(w - 5 - 12 * u, battery, 11 * u, u); // outline
    ctx.fillRect(w - 5 - 12 * u, battery + 5 * u, 11 * u, u);
    ctx.fillRect(w - 5 - 12 * u, battery, u, 6 * u);
    ctx.fillRect(w - 5 - 2 * u, battery, u, 6 * u);
    ctx.fillRect(w - 5 - u, battery + 2 * u, u, 2 * u); // terminal
    ctx.fillRect(w - 5 - 10 * u, battery + 2 * u, 6 * u, 2 * u); // charge

    // The menu: icon on the right, label to its left, the selection bar on the active row.
    for (let i = 1; i < rows.length; i++) {
      const [top, bottom] = band(i);
      const tall = bottom - top;
      const selected = i === active;
      if (selected) highlight(top + 1, bottom - 1);
      else {
        ctx.fillStyle = "rgba(255,255,255,0.08)";
        ctx.fillRect(4, bottom - 1, w - 8, 1);
      }
      const scale = Math.max(1, Math.floor((tall - 3) / 9));
      const right = w - 6;
      const color = selected ? "#1a0f00" : "#ffffff";
      if (ICONS[i]) icon(ICONS[i], right, top + Math.round((tall - 8 * scale) / 2), scale, selected ? "#3a1f00" : "#9fe0ff");
      const label = LABELS[rows[i].dataset.text || ""] || rows[i].dataset.text || "";
      const size = Math.round(tall * 0.68);
      const baseline = top + Math.round(tall * 0.72);
      if (!selected) text(label, right - 8 * scale - 4 + 1, baseline + 1, size, "#00103a", "right"); // drop shadow
      text(label, right - 8 * scale - 4, baseline, size, color, "right");
    }

    // A scrollbar on the left that tracks the selection.
    const [first] = band(1);
    const [, last] = band(rows.length - 1);
    ctx.fillStyle = "rgba(255,255,255,0.18)";
    ctx.fillRect(1, first + 1, 1, last - first - 2);
    if (active > 0) {
      const [top, bottom] = band(active);
      ctx.fillStyle = "#ffd27a";
      ctx.fillRect(0, top + 1, 3, bottom - top - 2);
    }

    // Down to 4096 colours, dithered.
    const frame = ctx.getImageData(0, 0, w, h);
    const d = frame.data;
    for (let y = 0; y < h; y++)
      for (let x = 0; x < w; x++) {
        const t = (BAYER[(y & 3) * 4 + (x & 3)] + 0.5) / 16 - 0.5;
        const i = (y * w + x) * 4;
        for (let c = 0; c < 3; c++) d[i + c] = Math.min(15, Math.max(0, Math.round((d[i + c] / 255) * 15 + t))) * 17;
      }
    ctx.putImageData(frame, 0, 0);
    present();
  }

  function setActive(index: number) {
    if (index === active || !running) return;
    active = index;
    draw();
  }

  // Leaving one row fires before entering the next; wait a frame before falling back to
  // the status bar, so crossing rows doesn't flash it.
  rows.forEach((row, index) => {
    row.addEventListener("mouseenter", () => {
      hovered = index;
      setActive(index);
    });
    row.addEventListener("mouseleave", () => {
      if (hovered !== index) return;
      hovered = null;
      requestAnimationFrame(() => hovered === null && setActive(0));
    });
  });

  // The backlight: dims after a while untouched, wakes on any movement.
  window.addEventListener("mousemove", () => {
    lastMove = performance.now();
    if (running) root.classList.remove("flip-dim");
  });

  let pending = 0;
  window.addEventListener("resize", () => {
    if (!running) return;
    cancelAnimationFrame(pending);
    pending = requestAnimationFrame(() => {
      layout();
      draw();
    });
  });

  return {
    start() {
      if (running || !window.innerWidth || !window.innerHeight) return;
      running = true;
      active = hovered ?? 0;
      lastMove = performance.now();
      layout();
      draw();
      root.classList.add("flip-ready");
      // The clock and the signal bars, and the backlight timeout.
      ticker = window.setInterval(() => {
        draw();
        root.classList.toggle("flip-dim", performance.now() - lastMove > BACKLIGHT_MS);
      }, 2000);
    },
    stop() {
      running = false;
      clearInterval(ticker);
      root.classList.remove("flip-ready", "flip-dim");
    },
  };
}
