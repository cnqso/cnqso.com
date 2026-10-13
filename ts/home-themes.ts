// Homepage themes. Clicking the CNQSO.COM title cycles to the next look;
// the links and layout stay the same, only the styling changes.
// The choice persists in localStorage (applied early by an inline <head> script).

const THEMES = ["brut", "liquid", "bauhaus", "aero", "crochet", "flip"];
const THEME_KEY = "cnqso-theme";
// One face per theme, so a switch can re-fit rows once the new font has loaded.
const THEME_FONTS: Record<string, string> = {
  brut: '1em "Coral Pixels"',
  liquid: '900 1em "Unbounded"',
  bauhaus: '1em "Archivo Black"',
  aero: '700 1em "Helvetica Neue", Helvetica, Arial',
  crochet: '1em "Archivo Black"',
  flip: "1em sans-serif",
};
// Heavier themes live in their own modules. Nothing is fetched on a plain visit; the
// first theme switch preloads and prepares them in idle time, so they start instantly.
const SCENES: Record<string, { module: string; create: string }> = {
  aero: { module: "/static/js/aero.js?v=26", create: "createAero" },
  crochet: { module: "/static/js/crochet.js?v=8", create: "createCrochet" },
  flip: { module: "/static/js/flip.js?v=6", create: "createFlip" },
};

// ---- Liquid shader: domain-warped fbm, looped by moving time around a circle ----

const WARP = 1.0;
const LOOP_MS = 7600;
// Epicycle: a surge of stronger turbulence boils up from below the screen, leaves a
// churning wake, then calms. Its period doesn't divide the base loop, so the two
// drift against each other and the motion never exactly repeats.
const SURGE_MS = 11300;
const SURGE_WARP = 2.2; // extra warp at the surge front
const MAX_PIXELS = 350000; // render small and let CSS upscale; it's soft by design

const VERTEX_SHADER = "attribute vec2 p;void main(){gl_Position=vec4(p,0.,1.);}";
const FRAGMENT_SHADER = `precision highp float;
uniform vec2 R;
uniform float T, W, S, SW;
float h2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float n(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);
  return mix(mix(h2(i),h2(i+vec2(1,0)),f.x),mix(h2(i+vec2(0,1)),h2(i+1.),f.x),f.y);}
float fbm(vec2 p){float s=0.,a=.5;mat2 m=mat2(1.6,1.2,-1.2,1.6);
  for(int i=0;i<5;i++){s+=a*n(p);p=m*p;a*=.5;}return s;}
// Surge strength at this pixel. The front rises from below the screen over the first
// 60% of the cycle and fades out by 75%, so it is zero when S wraps (no visible seam).
float surge(vec2 f){
  float front=mix(-.3,1.3,S/.6);
  float y=f.y+(n(vec2(f.x*3.,S*4.))-.5)*.3; // ragged, shifting front edge
  float dy=y-front;
  float I=exp(-dy*dy/.015);
  if(dy<0.)I=max(I,.6*exp(dy/.3)); // churning wake below the front
  return I*(1.-smoothstep(.5,.75,S));}
float H(vec2 p,float I,out vec2 q){
  // Inside the surge, sample noise that slides downward so features boil upward.
  p+=I*1.1*(vec2(fbm(p*1.4+vec2(0.,-S*6.)),fbm(p*1.4+vec2(4.7,-S*6.)))-.5);
  float w=W+I*SW;
  float a=6.2831853*T;vec2 o=vec2(cos(a),sin(a))*.9;
  q=vec2(fbm(p+o),fbm(p+vec2(5.2,1.3)+o.yx));
  vec2 r=vec2(fbm(p+w*q+vec2(1.7,9.2)),fbm(p+w*q+vec2(8.3,2.8)));
  return fbm(p+w*r);}
void main(){
  vec2 uv=gl_FragCoord.xy/R.y*2.2;vec2 q,qq;float e=.004;
  float I=surge(gl_FragCoord.xy/R);
  float h=H(uv,I,q),hx=H(uv+vec2(e,0),I,qq),hy=H(uv+vec2(0,e),I,qq);
  vec3 N=normalize(vec3((h-hx)/e,(h-hy)/e,.9));vec3 L=normalize(vec3(-.5,.6,.7));
  float d=max(dot(N,L),0.);
  float sp=pow(max(dot(reflect(-L,N),vec3(0,0,1)),0.),24.);
  float env=.5+.5*sin(N.x*9.+N.y*5.+h*6.);
  vec3 col=mix(vec3(.0,.09,.05),vec3(.05,.75,.25),smoothstep(.25,.75,h));
  col=mix(col,vec3(.45,1.,.85),env*smoothstep(.3,.9,length(q)));
  col*=.45+.75*d;col+=sp*vec3(.75,1.,.95);
  gl_FragColor=vec4(col,1.);}`;

class Liquid {
  private canvas: HTMLCanvasElement;
  private gl: WebGLRenderingContext | null = null;
  private uniforms: Record<string, WebGLUniformLocation | null> = {};
  private frame = 0;
  private running = false;
  private still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // The finger's own copy of the liquid, used where it covers the black active row.
  private cursor = document.getElementById("cursor");
  private cursorCanvas = document.querySelector<HTMLCanvasElement>("#cursor .cursor-liquid");

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
    window.addEventListener("resize", () => {
      if (this.running) this.draw(performance.now());
    });
    window.addEventListener("mousemove", () => {
      if (this.running && this.still) this.draw(performance.now());
    });
  }

  start() {
    if (this.running) return;
    if (!this.gl && !this.init()) return; // no WebGL: the CSS background color shows instead
    this.running = true;
    this.frame = requestAnimationFrame((t) => this.tick(t));
  }

  stop() {
    this.running = false;
    const target = this.cursorCanvas;
    target?.getContext("2d")?.clearRect(0, 0, target.width, target.height);
    cancelAnimationFrame(this.frame);
  }

  private init(): boolean {
    const gl = this.canvas.getContext("webgl");
    if (!gl) return false;
    const program = gl.createProgram()!;
    const compile = (type: number, source: string) => {
      const shader = gl.createShader(type)!;
      gl.shaderSource(shader, source);
      gl.compileShader(shader);
      gl.attachShader(program, shader);
    };
    compile(gl.VERTEX_SHADER, VERTEX_SHADER);
    compile(gl.FRAGMENT_SHADER, FRAGMENT_SHADER);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return false;
    gl.useProgram(program);

    // One oversized triangle covers the screen.
    gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    const position = gl.getAttribLocation(program, "p");
    gl.enableVertexAttribArray(position);
    gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);

    for (const name of ["R", "T", "W", "S", "SW"]) this.uniforms[name] = gl.getUniformLocation(program, name);
    this.gl = gl;
    return true;
  }

  private tick(now: number) {
    if (!this.running) return;
    this.draw(now);
    if (!this.still) this.frame = requestAnimationFrame((t) => this.tick(t));
  }

  private draw(now: number) {
    const gl = this.gl!;
    const scale = Math.min(0.5, Math.sqrt(MAX_PIXELS / (window.innerWidth * window.innerHeight)));
    const width = Math.max(1, Math.round(window.innerWidth * scale));
    const height = Math.max(1, Math.round(window.innerHeight * scale));
    if (this.canvas.width !== width || this.canvas.height !== height) {
      this.canvas.width = width;
      this.canvas.height = height;
    }
    gl.viewport(0, 0, width, height);
    gl.uniform2f(this.uniforms.R, width, height);
    gl.uniform1f(this.uniforms.T, this.still ? 0 : (now % LOOP_MS) / LOOP_MS);
    gl.uniform1f(this.uniforms.W, WARP);
    gl.uniform1f(this.uniforms.S, this.still ? 0.75 : (now % SURGE_MS) / SURGE_MS);
    gl.uniform1f(this.uniforms.SW, SURGE_WARP);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    this.traceCursor();
  }

  // The black active row hides the liquid, so the finger's backdrop filter only sees
  // black there. Copy the liquid into the finger for just that overlap; CSS shows it
  // un-inverted, so the polarity flips wherever the finger is over black.
  // Must run right after drawArrays, while the WebGL frame is still readable.
  private traceCursor() {
    const target = this.cursorCanvas;
    if (!target || !this.cursor) return;
    const ctx = target.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, target.width, target.height);

    const finger = this.cursor.getBoundingClientRect();
    const row = document.querySelector(".row:hover") || document.querySelector(".row.title");
    if (!row || finger.width === 0) return;
    const black = row.getBoundingClientRect();
    const left = Math.max(finger.left, black.left);
    const top = Math.max(finger.top, black.top);
    const right = Math.min(finger.right, black.right);
    const bottom = Math.min(finger.bottom, black.bottom);
    if (right <= left || bottom <= top) return;

    const toLocal = target.width / finger.width;
    const toLiquid = this.canvas.width / window.innerWidth;
    ctx.save();
    ctx.beginPath();
    ctx.rect((left - finger.left) * toLocal, (top - finger.top) * toLocal, (right - left) * toLocal, (bottom - top) * toLocal);
    ctx.clip();
    ctx.drawImage(
      this.canvas,
      finger.left * toLiquid,
      finger.top * toLiquid,
      finger.width * toLiquid,
      finger.height * toLiquid,
      0,
      0,
      target.width,
      target.height,
    );
    ctx.restore();
  }
}

// ---- Theme cycling ----

(function () {
  const root = document.documentElement;
  const title = document.querySelector<HTMLAnchorElement>(".row.title");
  const canvas = document.getElementById("liquid") as HTMLCanvasElement | null;
  const liquid = canvas ? new Liquid(canvas) : null;
  const rows = Array.prototype.slice.call(document.querySelectorAll(".row")) as HTMLElement[];
  const scenes: Record<string, Promise<any>> = {};

  // Each scene draws into the canvas of the same id. If it fails to load, the theme's
  // CSS fallback (plain type) stays.
  function scene(name: string) {
    if (!scenes[name]) {
      const canvas = document.getElementById(name);
      const { module, create } = SCENES[name];
      scenes[name] = canvas
        ? import(module).then((loaded) => loaded[create](canvas, rows)).catch(() => null)
        : Promise.resolve(null);
    }
    return scenes[name];
  }

  // The inline fit() in index.html listens for resize; re-run it for the new font.
  function refit(theme: string) {
    window.dispatchEvent(new Event("resize"));
    if (document.fonts) {
      document.fonts.load(THEME_FONTS[theme]).then(() => window.dispatchEvent(new Event("resize")));
    }
  }

  function apply(theme: string) {
    root.dataset.theme = theme;
    if (theme === "liquid") liquid?.start();
    else liquid?.stop();
    refit(theme);
    for (const name in SCENES) {
      // Re-check on load: the theme may have changed again while the scene was loading.
      if (name === theme) scene(name).then((loaded) => root.dataset.theme === name && loaded?.start());
      else scenes[name]?.then((loaded) => loaded?.stop());
    }
  }

  let current = THEMES.indexOf(root.dataset.theme || "");
  if (current < 0) current = 0;
  apply(THEMES[current]);

  // Idle-time preload, once someone has shown they want to see themes.
  function preloadScenes() {
    const w = window as any;
    for (const name in SCENES) {
      if (scenes[name]) continue;
      if (w.requestIdleCallback) w.requestIdleCallback(() => scene(name), { timeout: 1500 });
      else setTimeout(() => scene(name), 300);
    }
  }

  title?.addEventListener("click", (event) => {
    // Let modified clicks (new tab, etc.) behave like a normal link.
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    preloadScenes();
    current = (current + 1) % THEMES.length;
    apply(THEMES[current]);
    try {
      localStorage.setItem(THEME_KEY, THEMES[current]);
    } catch {
      // storage unavailable: the theme still changes for this visit
    }
  });
})();
