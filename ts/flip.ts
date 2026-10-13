// Flip theme: the homepage as the main menu of a mid-2000s Arabic-language flip phone.
// A tiny screen (about 176 pixels across) in 4096 colours with ordered dithering,
// scaled up with hard pixels under an LCD grid. Legible or not, by now you know the top
// of the screen changes the theme. Loaded on demand by home-themes.ts.

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
const SCREEN_WIDTH = 176; // pixels across, like the phones of the day
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

// Ordered dithering down to 4 bits per channel, the colour depth of the era.
const BAYER = [0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5];

export async function createFlip(canvas: HTMLCanvasElement, rows: HTMLElement[]) {
  const root = document.documentElement;
  const ctx = canvas.getContext("2d", { willReadFrequently: true })!;
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
    px = Math.max(3, Math.round(window.innerWidth / SCREEN_WIDTH));
    w = Math.ceil(window.innerWidth / px);
    h = Math.ceil(window.innerHeight / px);
    canvas.width = ink.canvas.width = w;
    canvas.height = ink.canvas.height = h;
    canvas.style.width = `${w * px}px`;
    canvas.style.height = `${h * px}px`;
    root.style.setProperty("--px", `${px}px`);
  }

  // A menu row's band on the screen, in screen pixels.
  function band(index: number) {
    const box = rows[index].getBoundingClientRect();
    return [Math.round(box.top / px), Math.round(box.bottom / px)];
  }

  // Text the way the phone drew it: shaped by the browser, then crushed to 1-bit pixels.
  function text(str: string, x: number, baseline: number, size: number, color: string, align: CanvasTextAlign) {
    ink.clearRect(0, 0, w, h);
    ink.font = `bold ${size}px ${FONT}`;
    ink.direction = /[؀-ۿ]/.test(str) ? "rtl" : "ltr";
    ink.textAlign = align;
    ink.fillStyle = "#fff";
    ink.fillText(str, x, baseline);
    const mask = ink.getImageData(0, 0, w, h).data;
    ctx.fillStyle = color;
    for (let y = 0; y < h; y++)
      for (let x = 0; x < w; x++) if (mask[(y * w + x) * 4 + 3] > 120) ctx.fillRect(x, y, 1, 1);
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
    ctx.fillStyle = fg;
    for (let b = 0; b < 5; b++) {
      const barHeight = Math.round(((b + 1) / 5) * tall * 0.55);
      if (b < bars) ctx.fillRect(3 + b * 3, top + tall * 0.75 - barHeight, 2, barHeight);
      else ctx.fillRect(3 + b * 3, top + tall * 0.75 - 1, 2, 1);
    }
    const now = new Date();
    const clock = `${arabicNumber(now.getHours() % 12 || 12)}:${arabicNumber(now.getMinutes())}`;
    text(clock, 20, top + tall * 0.72, Math.round(tall * 0.55), fg, "left");
    text(LABELS[rows[0].dataset.text || ""] || "كنقسو", w / 2, top + tall * 0.72, Math.round(tall * 0.62), fg, "center");
    ctx.strokeStyle = fg;
    ctx.strokeRect(w - 16.5, top + tall * 0.35 + 0.5, 11, 6);
    ctx.fillRect(w - 5, top + tall * 0.35 + 2, 1, 3);
    ctx.fillRect(w - 15, top + tall * 0.35 + 2, 6, 3);

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
