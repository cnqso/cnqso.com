// Aero theme: the homepage letters as glossy, iridescent glass glyphs in a
// three.js scene, over a photographed Swiss valley with a painted, cel-shaded
// cumulonimbus erupting behind the peak, bloom and lens grime.
// Loaded on demand by home-themes.ts.
//
// The HTML rows stay on top as invisible links; every glyph is placed where its
// .ch span sits, so the layout (and the letter density) is exactly the same.
// three.js comes from the CDN via the import map in index.html, so it is typed `any`.
//
// Cursor: a rigged hand posed into the pointing gesture with a weirdly articulated
// idle. Master Hand (Brawl model, go/static/models/master-hand/) by default; if it
// fails to load, the open hand from the WebXR input profiles (MIT, Amazon).
//
// Backplate: "Alps Field" by Andreas Mischok, Poly Haven (CC0), reprojected from the
// panorama to a flat view (go/static/images/aero/). The glass reflects a CG studio,
// not the photo: real scenery, too-perfect lighting.

const THREE_SPEC = "three";
const ADDONS = "three/addons/";
const FONT_URL = "https://cdn.jsdelivr.net/npm/three@0.170.0/examples/fonts/helvetiker_bold.typeface.json";

const FOV = 30;
const BASELINE = 0.84; // baseline position within a row, in ems (matches the HTML line box)
const GLYPH_SCALE = 0.71; // Helvetiker caps are ~1em tall; HTML caps are ~0.72em
const BACKDROP_Z = -2500;
const PLATE_URL = "/static/images/aero/alps-valley.webp?v=1";
const SKY_MASK_URL = "/static/images/aero/alps-sky-mask.png?v=1"; // white where the photo is open sky
const PLATE_ASPECT = 16 / 9;
const PLATE_FOCUS_X = 0.56; // keep the snowy peak in frame on narrow screens

const MASTER_HAND_URL = "/static/models/master-hand/master-hand.glb?v=1";
const XR_HAND_URL =
  "https://cdn.jsdelivr.net/npm/@webxr-input-profiles/assets@1.0.19/dist/profiles/generic-hand/right.glb";
const HAND_PX = 165; // wrist to fingertip, as it appears on screen
const HAND_Z = 300; // well in front of the letters, even popped and wobbling
// The pointing pose, per knuckle base to tip: [curl toward the palm, splay, twist],
// in radians. Index out, the rest folded, thumb laid along the folded fingers.
const POINTING: Record<string, [number, number, number][]> = {
  index: [[0.06, 0, 0], [0.1, 0, 0], [0.06, 0, 0]],
  middle: [[1.45, 0, 0], [1.7, 0, 0], [0.9, 0, 0]],
  ring: [[1.5, 0, 0], [1.75, 0, 0], [0.9, 0, 0]],
  pinky: [[1.55, 0, 0], [1.75, 0, 0], [0.85, 0, 0]],
  thumb: [[-0.3, 0, 0], [0.3, 0, 0], [0.4, 0, 0]], // lifted clear of the folded fingers
};

// ---- Backdrop: photo plate, graded, with a painted cumulonimbus behind the peaks ----

const BACKDROP_VERTEX = `varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}`;
const BACKDROP_FRAGMENT = `
precision highp float;
varying vec2 vUv;
uniform sampler2D uPlate, uSky;
uniform vec4 uCover; // xy = uv scale, zw = uv offset (cover-fit the plate to the screen)
uniform float uTime, uAspect;
float h2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float n(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);
  return mix(mix(h2(i),h2(i+vec2(1,0)),f.x),mix(h2(i+vec2(0,1)),h2(i+1.),f.x),f.y);}
float fbm(vec2 p){float s=0.,a=.5;for(int i=0;i<5;i++){s+=a*n(p);p=p*2.03+17.1;a*=.5;}return s;}
vec3 lin(vec3 c){return pow(c,vec3(2.2));}

// Nyudogumo (the anime summer cumulonimbus): a fat pillar of stacked, bulbous lobes,
// broadest at the base and tapering to a rounded crown. Each lobe is a sphere; a pixel shows the frontmost sphere's
// surface, so every billow gets its own lit side, cel shadow and crease.
const int LOBES=34;
float hash(float n){return fract(sin(n)*43758.5453);}
vec4 lobe(int i){ // xy = centre, z = depth bias, w = radius
  float fi=float(i),t=fi/float(LOBES-1);
  float hw=mix(.21,.11,smoothstep(0.,.9,t)); // broad base tapering to the crown
  float x=(hash(fi*1.7)-.5)*2.*hw*.7;
  float y=mix(.40,.79,pow(t,.8))+sin(uTime*.05+fi)*.004;
  float r=mix(.14,.075,t)*(.8+.45*hash(fi*3.1+2.))*(1.+.03*sin(uTime*.3+fi*2.));
  float z=hash(fi*5.3+7.)*.06+(1.-abs(x)/hw)*.05;
  return vec4(.13*uAspect+x,y,z,r);}

void main(){
  vec2 uv=vUv*uCover.xy+uCover.zw;
  vec3 col=texture2D(uPlate,uv).rgb; // linear (sRGB texture)
  float sky=texture2D(uSky,uv).r;

  // A little too vivid, a little too clean: the uncanny product-render grade.
  float l=dot(col,vec3(.2126,.7152,.0722));
  col=mix(vec3(l),col,1.22);
  col*=mix(vec3(1.),vec3(.92,.98,1.08),sky);

  vec2 p=vec2((uv.x-.5)*uAspect,uv.y);

  // A few painted fair-weather puffs high in the sky.
  if(uv.y>.72&&sky>.002){
    float c=fbm(vec2(p.x*2.2+uTime*.006,uv.y*7.))*smoothstep(.72,.8,uv.y);
    float puff=smoothstep(.6,.66,c)*sky;
    col=mix(col,lin(mix(vec3(.78,.86,.97),vec3(1.),smoothstep(.62,.7,c))),puff*.8);
  }

  // The nyudogumo, cel-shaded and lit from the upper left like the photo's sun, shown
  // only through the sky mask so it sits behind the real mountains.
  float top=-1.,edge=-1.;vec3 N=vec3(0.,0.,1.);
  // Only pixels the cloud can reach pay for the 34-lobe loop (most of the screen doesn't).
  if(sky>.002&&abs(p.x-.13*uAspect)<.42&&uv.y>.25&&uv.y<.98)
  for(int i=0;i<LOBES;i++){
    vec4 L=lobe(i);vec2 d=p-L.xy;float q=L.w*L.w-dot(d,d);
    edge=max(edge,L.w-length(d));
    if(q>0.){float h=sqrt(q)+L.z;if(h>top){top=h;N=vec3(d,sqrt(q))/L.w;}}
  }
  if(edge>-.004){
    N.xy+=(vec2(fbm(p*34.+uTime*.01),fbm(p*34.+5.))-.5)*.5; // cauliflower micro-billows
    N=normalize(N);
    float k=dot(N,normalize(vec3(-.55,.6,.6)));
    vec3 cc=mix(vec3(.47,.54,.75),vec3(.70,.76,.91),smoothstep(.1,.15,k)); // crease to shadow
    cc=mix(cc,vec3(.98,.98,.96),smoothstep(.5,.55,k)); // sunlit
    cc+=vec3(1.,.82,.6)*.12*smoothstep(.85,.95,k); // warm hot spots
    cc=mix(cc*vec3(.86,.9,1.),cc,smoothstep(.40,.60,uv.y)); // bluer, hazier base
    col=mix(col,lin(cc)*.85,smoothstep(-.004,.002,edge)*sky);
  }

  gl_FragColor=vec4(col,1.);
}`;

// ---- Lens: chromatic aberration, vignette, grain ("over-shadered") ----

const LENS_SHADER = {
  uniforms: { tDiffuse: { value: null }, uTime: { value: 0 } },
  vertexShader: `varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}`,
  fragmentShader: `
    uniform sampler2D tDiffuse;uniform float uTime;varying vec2 vUv;
    float h(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
    void main(){
      vec2 d=vUv-.5;float r=dot(d,d);
      vec3 c=vec3(texture2D(tDiffuse,vUv+d*r*.035).r,texture2D(tDiffuse,vUv).g,texture2D(tDiffuse,vUv-d*r*.035).b);
      c*=1.-.45*smoothstep(.08,.5,r);
      c+=(h(vUv*1024.+fract(uTime)*91.)-.5)*.03;
      gl_FragColor=vec4(c,1.);
    }`,
};

interface Glyph {
  mesh: any;
  row: number;
  index: number;
  baseY: number;
  size: number;
}

export async function createAero(canvas: HTMLCanvasElement, rows: HTMLElement[]) {
  const load = (spec: string) => import(spec);
  const [
    THREE,
    { EffectComposer },
    { RenderPass },
    { UnrealBloomPass },
    { ShaderPass },
    { OutputPass },
    { FontLoader },
    { TextGeometry },
    { RoomEnvironment },
    { GLTFLoader },
  ] = await Promise.all([
    load(THREE_SPEC),
    load(ADDONS + "postprocessing/EffectComposer.js"),
    load(ADDONS + "postprocessing/RenderPass.js"),
    load(ADDONS + "postprocessing/UnrealBloomPass.js"),
    load(ADDONS + "postprocessing/ShaderPass.js"),
    load(ADDONS + "postprocessing/OutputPass.js"),
    load(ADDONS + "loaders/FontLoader.js"),
    load(ADDONS + "geometries/TextGeometry.js"),
    load(ADDONS + "environments/RoomEnvironment.js"),
    load(ADDONS + "loaders/GLTFLoader.js"),
  ]);
  const textures = new THREE.TextureLoader();
  const [font, plate, skyMask, handGltf] = await Promise.all([
    new FontLoader().loadAsync(FONT_URL),
    textures.loadAsync(PLATE_URL),
    textures.loadAsync(SKY_MASK_URL),
    new GLTFLoader()
      .loadAsync(MASTER_HAND_URL)
      .catch(() => new GLTFLoader().loadAsync(XR_HAND_URL))
      .catch(() => null), // no hand at all: keep the 2D finger
  ]);
  plate.colorSpace = THREE.SRGBColorSpace;

  const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: "high-performance" });
  // Resolution governor: start sharp, step down if frames drop, back up with headroom.
  const maxRatio = Math.min(window.devicePixelRatio, 1.5);
  let pixelRatio = maxRatio;
  renderer.setPixelRatio(pixelRatio);
  renderer.transmissionResolutionScale = 0.5; // the glass refraction buffer
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.05;

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(FOV, 1, 1, 12000);
  const pmrem = new THREE.PMREMGenerator(renderer);
  scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;

  const sunLight = new THREE.DirectionalLight(0xfff2dc, 2.4);
  sunLight.position.set(-900, 800, 700); // the photo's sun is up and to the left
  scene.add(sunLight, new THREE.HemisphereLight(0xbfe8ff, 0x5fbf4f, 0.9));

  // Backdrop plane, resized in layout() to overfill the view at its depth.
  const backdropMaterial = new THREE.ShaderMaterial({
    uniforms: {
      uPlate: { value: plate },
      uSky: { value: skyMask },
      uCover: { value: new THREE.Vector4(1, 1, 0, 0) },
      uTime: { value: 0 },
      uAspect: { value: PLATE_ASPECT },
    },
    vertexShader: BACKDROP_VERTEX,
    fragmentShader: BACKDROP_FRAGMENT,
    depthWrite: false,
  });
  const backdrop = new THREE.Mesh(new THREE.PlaneGeometry(1, 1), backdropMaterial);
  backdrop.position.z = BACKDROP_Z;
  scene.add(backdrop);

  // Glass letters: one material per row so the active row can turn Aero green.
  const glass = new THREE.Color(0xa8e8ff);
  const glassTint = new THREE.Color(0x0a8cff);
  const lime = new THREE.Color(0xa8ff4f);
  const limeTint = new THREE.Color(0x3fd000);
  const rowMaterials = rows.map(
    () =>
      new THREE.MeshPhysicalMaterial({
        color: glass.clone(),
        roughness: 0.04,
        metalness: 0,
        transmission: 0.78,
        thickness: 0.45,
        ior: 1.5,
        iridescence: 1,
        iridescenceIOR: 1.35,
        iridescenceThicknessRange: [180, 720],
        clearcoat: 1,
        clearcoatRoughness: 0.03,
        attenuationColor: glassTint.clone(),
        attenuationDistance: 0.35,
        envMapIntensity: 2.2,
        emissive: new THREE.Color(0x000000),
      }),
  );

  const geometries = new Map<string, any>();
  function glyphGeometry(ch: string) {
    let geometry = geometries.get(ch);
    if (!geometry) {
      geometry = new TextGeometry(ch, {
        font,
        size: 1,
        depth: 0.3,
        curveSegments: 6,
        bevelEnabled: true,
        bevelThickness: 0.06,
        bevelSize: 0.03,
        bevelSegments: 4,
      });
      geometry.computeBoundingBox();
      const box = geometry.boundingBox;
      geometry.translate(-(box.min.x + box.max.x) / 2, 0, -0.15);
      geometry.computeVertexNormals();
      geometries.set(ch, geometry);
    }
    return geometry;
  }

  // Built a few letters at a time, yielding between batches, so a background preload
  // never stalls whatever theme is on screen.
  const yieldFrame = () => new Promise((resolve) => setTimeout(resolve, 0));
  const glyphs: Glyph[] = [];
  for (let r = 0; r < rows.length; r++) {
    for (const span of Array.prototype.slice.call(rows[r].children) as HTMLElement[]) {
      const ch = span.textContent || "";
      if (!ch.trim()) continue;
      const fresh = !geometries.has(ch);
      const mesh = new THREE.Mesh(glyphGeometry(ch), rowMaterials[r]);
      scene.add(mesh);
      glyphs.push({ mesh, row: r, index: glyphs.length, baseY: 0, size: 1 });
      (mesh as any).userData.span = span;
      if (fresh && geometries.size % 4 === 0) await yieldFrame();
    }
  }

  const hand = handGltf ? createHand(THREE, handGltf.scene) : null;
  if (hand) scene.add(hand.root);

  const composer = new EffectComposer(renderer);
  composer.addPass(new RenderPass(scene, camera));
  const bloom = new UnrealBloomPass(new THREE.Vector2(1, 1), 0.4, 0.45, 1.0);
  composer.addPass(bloom);
  const lens = new ShaderPass(LENS_SHADER);
  composer.addPass(lens);
  composer.addPass(new OutputPass());

  let width = 1;
  let height = 1;
  let distance = 1;

  // Match the HTML: one world unit is one CSS pixel on the z = 0 plane.
  function layout() {
    width = window.innerWidth;
    height = window.innerHeight;
    renderer.setSize(width, height, false);
    composer.setSize(width, height);
    bloom.setSize(width, height);
    camera.aspect = width / height;
    distance = height / 2 / Math.tan(((FOV / 2) * Math.PI) / 180);
    hand?.resize((HAND_PX * (distance - HAND_Z)) / distance);
    camera.position.set(0, 0, distance);
    camera.updateProjectionMatrix();

    const span = ((distance - BACKDROP_Z) / distance) * 1.15; // overfill for camera drift
    backdrop.scale.set(width * span, height * span, 1);
    const aspect = width / height;
    const cover = backdropMaterial.uniforms.uCover.value;
    if (aspect < PLATE_ASPECT) {
      const sx = aspect / PLATE_ASPECT;
      cover.set(sx, 1, Math.min(1 - sx, Math.max(0, PLATE_FOCUS_X - sx / 2)), 0);
    } else {
      const sy = PLATE_ASPECT / aspect;
      cover.set(1, sy, 0, (1 - sy) * 0.55);
    }

    for (const glyph of glyphs) {
      const span = glyph.mesh.userData.span as HTMLElement;
      const row = span.parentElement as HTMLElement;
      const fontSize = parseFloat(row.style.fontSize) || row.clientHeight;
      const rect = span.getBoundingClientRect();
      const rowTop = row.getBoundingClientRect().top;
      glyph.size = fontSize * GLYPH_SCALE;
      glyph.baseY = height / 2 - (rowTop + fontSize * BASELINE);
      glyph.mesh.position.x = rect.left + rect.width / 2 - width / 2;
      glyph.mesh.scale.setScalar(glyph.size);
    }
  }

  // The active row mirrors the CSS: the hovered row, else the title.
  let hovered: number | null = null;
  let dirty = true; // reduced motion: only redraw when something changes
  rows.forEach((row, r) => {
    row.addEventListener("mouseenter", () => {
      hovered = r;
      dirty = true;
    });
    row.addEventListener("mouseleave", () => {
      if (hovered === r) hovered = null;
      dirty = true;
    });
  });
  const pop: number[] = rows.map((_, r) => (r === 0 ? 1 : 0));

  // Pointer position for the hand (no parallax: the camera ignores it).
  let pointerX = 0;
  let pointerY = 0;
  let pointerIn = false;
  window.addEventListener("mousemove", (event) => {
    pointerX = event.clientX;
    pointerY = event.clientY;
    pointerIn = true;
    dirty = true;
  });
  document.addEventListener("mouseleave", () => {
    pointerIn = false;
    dirty = true;
  });

  let running = false;
  let frame = 0;
  let last = performance.now();
  const clock = { t: 0 };

  function render(now: number) {
    const dt = Math.min(0.05, (now - last) / 1000);
    last = now;
    if (!still) clock.t += dt;
    const t = clock.t;
    const active = hovered ?? 0;

    // Gentle handheld drift; z = 0 stays registered with the HTML.
    camera.position.x = still ? 0 : Math.sin(t * 0.21) * 14;
    camera.position.y = still ? 0 : Math.sin(t * 0.17) * 9;
    camera.lookAt(0, 0, 0);
    camera.updateMatrixWorld();

    if (hand) {
      hand.root.visible = pointerIn;
      if (pointerIn) hand.update(t, pointerTarget(pointerX, pointerY));
    }

    rowMaterials.forEach((material, r) => {
      const target = r === active ? 1 : 0;
      pop[r] = still ? target : pop[r] + (target - pop[r]) * Math.min(1, dt * 9);
      material.color.lerpColors(glass, lime, pop[r]);
      material.attenuationColor.lerpColors(glassTint, limeTint, pop[r]);
      material.emissive.setRGB(0.05 * pop[r], 0.14 * pop[r], 0.02 * pop[r]);
    });

    for (const glyph of glyphs) {
      const p = pop[glyph.row];
      const i = glyph.index;
      const mesh = glyph.mesh;
      mesh.position.y = glyph.baseY + Math.sin(t * 1.3 + i * 0.9) * glyph.size * 0.02;
      mesh.position.z = p * glyph.size * 0.55 + Math.sin(t * 1.1 + i * 0.55) * glyph.size * 0.08;
      mesh.rotation.y = Math.sin(t * 0.7 + i * 1.3) * 0.14 + p * Math.sin(t * 3 + i) * 0.08;
      mesh.rotation.x = Math.sin(t * 0.9 + i * 0.4) * 0.06;
    }

    backdropMaterial.uniforms.uTime.value = t;
    lens.uniforms.uTime.value = t;
    composer.render();
  }

  // Where the pointer's ray crosses the hand's plane, so the fingertip sits on the hotspot.
  const ray = new THREE.Vector3();
  const hotspot = new THREE.Vector3();
  function pointerTarget(x: number, y: number) {
    ray.set((x / width) * 2 - 1, -(y / height) * 2 + 1, 0.5).unproject(camera).sub(camera.position).normalize();
    return hotspot.copy(camera.position).addScaledVector(ray, (HAND_Z - camera.position.z) / ray.z);
  }

  // Every ~1.5 s: below ~52 fps, drop resolution a notch; with headroom for a few
  // windows running, step back up toward the sharp ceiling.
  let windowStart = 0;
  let windowFrames = 0;
  let calm = 0;
  function govern(now: number) {
    if (!windowStart) windowStart = now;
    windowFrames++;
    if (now - windowStart < 1500) return;
    const frameMs = (now - windowStart) / windowFrames;
    windowStart = now;
    windowFrames = 0;
    let next = pixelRatio;
    if (frameMs > 19) {
      next = Math.max(0.6, pixelRatio - 0.15);
      calm = 0;
    } else if (frameMs < 17.5 && ++calm >= 3) {
      next = Math.min(maxRatio, pixelRatio + 0.1);
      calm = 0;
    }
    if (next !== pixelRatio) {
      pixelRatio = next;
      renderer.setPixelRatio(pixelRatio);
      composer.setPixelRatio(pixelRatio);
      layout();
    }
  }

  function tick(now: number) {
    if (!running) return;
    if (!still || dirty) {
      render(now);
      if (!still) govern(now);
    }
    dirty = false;
    frame = requestAnimationFrame(tick);
  }

  // Warm-up: upload textures and compile every shader (the hand too) now, off the main
  // thread where the browser allows, so the first visible frame is instant.
  layout();
  renderer.initTexture(plate);
  renderer.initTexture(skyMask);
  if (hand) hand.root.visible = true;
  await renderer.compileAsync(scene, camera);
  composer.render(); // the post passes compile on first use
  if (hand) hand.root.visible = false;

  window.addEventListener("resize", () => {
    if (!running) return;
    layout();
    render(performance.now());
  });

  return {
    start() {
      if (running) return;
      running = true;
      layout();
      last = performance.now();
      windowStart = 0;
      windowFrames = 0;
      render(last);
      document.documentElement.classList.add("aero-ready");
      if (hand) document.documentElement.classList.add("aero-hand");
      frame = requestAnimationFrame(tick);
    },
    stop() {
      running = false;
      cancelAnimationFrame(frame);
      document.documentElement.classList.remove("aero-ready", "aero-hand");
    },
  };
}

// ---- The hand cursor ----

// One knuckle: curl toward the palm, splay sideways, twist about the bone.
type Joint = [number, number, number];

interface Rig {
  forward: [string, string]; // joints along the hand, wrist to knuckles
  side: [string, string]; // joints across the hand, pinky side to index side
  wrist: string;
  tip: string; // the index fingertip, pinned to the pointer
  pose(joints: Record<string, Joint[]>): void; // per finger, base to tip

}

// Master Hand (Brawl rig): a proper bone hierarchy, so a curl is a local x rotation.
function masterRig(THREE: any, bone: (name: string) => any): Rig {
  const fingers: Record<string, string[]> = {
    index: ["F1stNa", "F1stNb", "F1stNc"],
    middle: ["F2ndNa", "F2ndNb", "F2ndNc"],
    ring: ["F3rdNa", "F3rdNb", "F3rdNc"],
    pinky: ["F4thNa", "F4thNb", "F4thNc"],
    thumb: ["ThumbNa", "ThumbNb", "ThumbNc"],
  };
  const rest: Record<string, any> = {};
  for (const names of Object.values(fingers)) for (const n of names) rest[n] = bone(n).quaternion.clone();
  const euler = new THREE.Euler(0, 0, 0, "XZY");
  const bend = new THREE.Quaternion();
  return {
    forward: ["HandN", "F2ndNa"],
    side: ["F4thNa", "F1stNa"],
    wrist: "WristN",
    tip: "F1stEfN",
    pose(joints) {
      // Bones run along local y: curl about x, splay about z, twist about y.
      for (const finger in fingers) {
        fingers[finger].forEach((n, i) => {
          const [curl, splay, twist] = joints[finger][i] ?? [0, 0, 0];
          bone(n).quaternion.copy(rest[n]).multiply(bend.setFromEuler(euler.set(curl, twist, splay)));
        });
      }
    },
  };
}

// WebXR hand: no bone hierarchy (every joint is positioned independently), so bending
// a joint here rotates every joint past it on that finger.
function xrRig(THREE: any, bone: (name: string) => any): Rig {
  const prefix: Record<string, string> = {
    index: "index-finger",
    middle: "middle-finger",
    ring: "ring-finger",
    pinky: "pinky-finger",
  };
  const chains: Record<string, string[]> = {
    thumb: ["thumb-metacarpal", "thumb-phalanx-proximal", "thumb-phalanx-distal", "thumb-tip"],
  };
  for (const f in prefix) {
    chains[f] = ["metacarpal", "phalanx-proximal", "phalanx-intermediate", "phalanx-distal", "tip"].map(
      (j) => `${prefix[f]}-${j}`,
    );
  }
  const rest: Record<string, { p: any; q: any }> = {};
  for (const names of Object.values(chains))
    for (const n of names) rest[n] = { p: bone(n).position.clone(), q: bone(n).quaternion.clone() };
  const local = new THREE.Quaternion();
  const turn = new THREE.Quaternion();
  const inverse = new THREE.Quaternion();
  const euler = new THREE.Euler(0, 0, 0, "XYZ");
  const offset = new THREE.Vector3();
  return {
    forward: ["wrist", "middle-finger-phalanx-proximal"],
    side: ["pinky-finger-phalanx-proximal", "index-finger-phalanx-proximal"],
    wrist: "wrist",
    tip: "index-finger-tip",
    pose(joints) {
      // Joints point down -z: curl is negative about x, splay about y, twist about z.
      for (const finger in chains) {
        const names = chains[finger];
        const first = finger === "thumb" ? 0 : 1;
        const P = names.map((n) => rest[n].p.clone());
        const Q = names.map((n) => rest[n].q.clone());
        joints[finger].forEach(([curl, splay, twist], k) => {
          const j = first + k;
          local.setFromEuler(euler.set(-curl, splay, twist));
          turn.copy(Q[j]).multiply(local).multiply(inverse.copy(Q[j]).invert()); // the local bend, in rig space
          for (let m = j; m < names.length; m++) {
            Q[m].premultiply(turn);
            if (m > j) {
              offset.copy(P[m]).sub(P[j]).applyQuaternion(turn);
              P[m].copy(P[j]).add(offset);
            }
          }
        });
        names.forEach((n, i) => {
          bone(n).position.copy(P[i]);
          bone(n).quaternion.copy(Q[i]);
        });
      }
    },
  };
}

function createHand(THREE: any, model: any) {
  const bone = (name: string) => model.getObjectByName(name);
  const master = !!bone("F1stNa");
  model.traverse((node: any) => {
    if (!node.isMesh) return;
    node.frustumCulled = false;
    // A white glove: soft satin, cool shadows. Keeps the model's own texture detail.
    node.material = new THREE.MeshPhysicalMaterial({
      map: node.material?.map ?? null,
      color: master ? 0xf2f1f6 : 0xc9c6c8,
      roughness: 0.5,
      sheen: 0.6,
      sheenColor: new THREE.Color(0xffffff),
      sheenRoughness: 0.4,
      clearcoat: 0.15,
      envMapIntensity: 0.5,
    });
  });
  const rig = master ? masterRig(THREE, bone) : xrRig(THREE, bone);

  // Orient the rest pose: fingers to screen-left (tilted a little toward the viewer),
  // index side up. For a right hand that shows the palm, like Master Hand.
  model.updateMatrixWorld(true);
  const at = (name: string) => bone(name).getWorldPosition(new THREE.Vector3());
  const forward = at(rig.forward[1]).sub(at(rig.forward[0])).normalize();
  const side = at(rig.side[1]).sub(at(rig.side[0]));
  side.addScaledVector(forward, -side.dot(forward)).normalize();
  const from = new THREE.Matrix4().makeBasis(forward, side, forward.clone().cross(side));
  const toForward = new THREE.Vector3(-1, -0.12, 0.35).normalize();
  const toSide = new THREE.Vector3(0, 1, 0).addScaledVector(toForward, -toForward.y).normalize();
  const to = new THREE.Matrix4().makeBasis(toForward, toSide, toForward.clone().cross(toSide));
  const base = new THREE.Quaternion().setFromRotationMatrix(to.multiply(from.transpose()));

  const root = new THREE.Group();
  root.add(model);
  const unit = 1 / at(rig.tip).distanceTo(at(rig.wrist));
  root.visible = false;

  const tip = new THREE.Vector3();
  const sway = new THREE.Quaternion();
  const euler = new THREE.Euler();
  const pose: Record<string, Joint[]> = {};
  const fingers = ["thumb", "index", "middle", "ring", "pinky"];

  // A soft bump once per period: an involuntary squeeze or twitch.
  const pulse = (t: number, period: number, width: number) => {
    const p = ((t % period) + period) % period;
    return p < width ? Math.pow(Math.sin((Math.PI * p) / width), 2) : 0;
  };
  // Smooth wandering in -1..1 that never quite repeats.
  const wander = (t: number, seed: number) => 0.6 * Math.sin(t * 0.9 + seed) + 0.4 * Math.sin(t * 1.53 + seed * 2.3);

  function update(t: number, target: any) {
    // Weirdly articulated: every knuckle wanders on its own, fingertips often against
    // their bases (double-jointed), fingers drift apart and twist. Now and then a folded
    // finger unfurls part-way with a hyperextended tip and folds back, the index crooks,
    // the thumb taps.
    fingers.forEach((finger, k) => {
      const seed = k * 7.31;
      const joints: Joint[] = POINTING[finger].map(([curl, splay, twist], i) => [
        curl + 0.09 * wander(t * (1 + i * 0.37), seed + i * 2.1) * (i === 2 ? -1.4 : 1),
        splay + (i === 0 ? 0.13 * wander(t * 0.45, seed + 11) : 0.05 * wander(t * 0.7, seed + i * 3.7)),
        twist + 0.09 * wander(t * 0.6, seed + i * 5.2) * (i / 2),
      ]);
      if (finger === "index") {
        const crook = pulse(t, 6.1, 1.2);
        joints[0][0] += 0.25 * crook;
        joints[1][0] += 0.8 * crook;
        joints[2][0] += 0.6 * crook + 0.025 * Math.sin(t * 23);
      } else if (finger === "thumb") {
        const tap = pulse(t - 1.3, 4.3, 0.35);
        joints[1][0] += 0.35 * tap;
        joints[2][0] += 0.25 * tap;
        joints[0][1] += 0.2 * wander(t * 0.4, 3);
      } else {
        const unfurl = pulse(t + k * 2.9, [11.7, 9.1, 7.3][k - 2], 1.9);
        joints[0][0] -= 1.0 * unfurl;
        joints[1][0] -= 1.1 * unfurl;
        joints[2][0] -= 1.3 * unfurl; // past straight: hyperextended
        joints[0][1] += 0.2 * unfurl * (k - 3);
      }
      pose[finger] = joints;
    });
    rig.pose(pose);

    const jerk = pulse(t, 13.3, 0.22);
    euler.set(Math.sin(t * 0.31) * 0.05, Math.sin(t * 0.23) * 0.06, Math.sin(t * 0.4) * 0.04 + jerk * 0.09);
    root.quaternion.copy(base).premultiply(sway.setFromEuler(euler));

    // Pin the index fingertip to the pointer.
    root.position.set(0, 0, 0);
    root.updateMatrixWorld(true);
    bone(rig.tip).getWorldPosition(tip);
    root.position.copy(target).sub(tip);
  }

  // World size from layout(), so the hand keeps its screen size at any depth.
  const resize = (worldSize: number) => root.scale.setScalar(worldSize * unit);

  return { root, update, resize };
}
