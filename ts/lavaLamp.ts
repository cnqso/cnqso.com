interface ScreenInterface {
  canvasWidth: number;
  canvasHeight: number;
  left: number;
  top: number;
  elem: HTMLElement | HTMLCanvasElement;
  ctx?: CanvasRenderingContext2D;
  callback?: (() => void) | null;
  init(elementId: string, callback?: (() => void) | null, shouldResize?: boolean): ScreenInterface;
  resize(): void;
}

interface Vector {
  x: number;
  y: number;
  magnitude: number;
  computed: number;
  force: number;
  add(vector: Vector): Vector;
}

interface Ball {
  vel: Vector;
  pos: Vector;
  size: number;
  width: number;
  height: number;
  move(): void;
}

interface MetaballField {
  step: number;
  width: number;
  height: number;
  wh: number;
  sx: number;
  sy: number;
  paint: boolean;
  metaFill: CanvasGradient;
  plx: number[];
  ply: number[];
  mscases: number[];
  ix: number[];
  grid: Vector[];
  balls: Ball[];
  iter: number;
  sign: number;
  computeForce(x: number, y: number, index?: number): number;
  marchingSquares(coords: [number, number, number]): [number, number, number] | false;
  renderMetaballs(): void;
}

const lavaLamp = (): void => {
  const lavaAnimation = (function (): { run: () => void } {
    let metaballField: MetaballField;
    let canvas: ScreenInterface;

    const screenModule: ScreenInterface = {
      canvasWidth: 0,
      canvasHeight: 0,
      left: 0,
      top: 0,
      elem: null as any,
      ctx: undefined,
      callback: null,

      init: function (elementId: string, callback?: (() => void) | null, shouldResize?: boolean): ScreenInterface {
        this.elem = document.getElementById(elementId)!;
        this.callback = callback || null;

        if (this.elem.tagName === "CANVAS") {
          this.ctx = (this.elem as HTMLCanvasElement).getContext("2d")!;
        }

        window.addEventListener(
          "resize",
          () => {
            this.resize();
          },
          false
        );

        this.elem.onselectstart = (): boolean => false;
        (this.elem as any).ondragstart = (): boolean => false;

        if (shouldResize) {
          this.resize();
        }

        return this;
      },

      resize: function (): void {
        let element: HTMLElement | null = this.elem as HTMLElement;
        this.canvasWidth = element.offsetWidth;
        this.canvasHeight = element.offsetHeight;
        this.left = 0;
        this.top = 0;

        while (element !== null) {
          this.left += element.offsetLeft;
          this.top += element.offsetTop;
          element = element.offsetParent as HTMLElement;
        }

        if (this.ctx) {
          (this.elem as HTMLCanvasElement).width = this.canvasWidth;
          (this.elem as HTMLCanvasElement).height = this.canvasHeight;
        }

        if (this.callback) {
          this.callback();
        }
      }
    };

    class Vector2D implements Vector {
      x: number;
      y: number;
      magnitude: number;
      computed: number = 0;
      force: number = 0;

      constructor(x: number, y: number) {
        this.x = x;
        this.y = y;
        this.magnitude = x * x + y * y;
      }

      add(vector: Vector): Vector {
        return new Vector2D(this.x + vector.x, this.y + vector.y);
      }
    }

    class LavaBall implements Ball {
      vel: Vector;
      pos: Vector;
      size: number;
      width: number;
      height: number;

      constructor(field: { width: number; height: number; wh: number }) {
        const minVel = 0.2;
        const maxVel = 0.45;
        const minSize = 0.1;
        const maxSize = 1.5;

        this.vel = new Vector2D(
          (Math.random() > 0.5 ? 1 : -1) * (minVel + 0.25 * Math.random()),
          (Math.random() > 0.5 ? 1 : -1) * (minVel + Math.random())
        );

        this.pos = new Vector2D(
          0.2 * field.width + Math.random() * field.width * 0.6,
          0.2 * field.height + Math.random() * field.height * 0.6
        );

        this.size = field.wh / 15 + (Math.random() * (maxSize - minSize) + minSize) * (field.wh / 15);
        this.width = field.width;
        this.height = field.height;
      }

      move(): void {
        if (this.pos.x >= this.width - this.size) {
          if (this.vel.x > 0) {
            this.vel.x = -this.vel.x;
          }
          this.pos.x = this.width - this.size;
        } else if (this.pos.x <= this.size) {
          if (this.vel.x < 0) {
            this.vel.x = -this.vel.x;
          }
          this.pos.x = this.size;
        }

        if (this.pos.y >= this.height - this.size) {
          if (this.vel.y > 0) {
            this.vel.y = -this.vel.y;
          }
          this.pos.y = this.height - this.size;
        } else if (this.pos.y <= this.size) {
          if (this.vel.y < 0) {
            this.vel.y = -this.vel.y;
          }
          this.pos.y = this.size;
        }

        this.pos = this.pos.add(this.vel);
      }
    }

    class MetaballRenderer implements MetaballField {
      step: number = 5;
      width: number;
      height: number;
      wh: number;
      sx: number;
      sy: number;
      paint: boolean = false;
      metaFill: CanvasGradient;
      plx: number[] = [0, 0, 1, 0, 1, 1, 1, 1, 1, 1, 0, 1, 0, 0, 0, 0];
      ply: number[] = [0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 1, 0, 1, 0, 1];
      mscases: number[] = [0, 3, 0, 3, 1, 3, 0, 3, 2, 2, 0, 2, 1, 1, 0];
      ix: number[] = [1, 0, -1, 0, 0, 1, 0, -1, -1, 0, 1, 0, 0, 1, 1, 0, 0, 0, 1, 1];
      grid: Vector[] = [];
      balls: Ball[] = [];
      iter: number = 0;
      sign: number = 1;

      constructor(width: number, height: number, ballCount: number, color1: string, color2: string) {
        this.width = width;
        this.height = height;
        this.wh = Math.min(width, height);
        this.sx = Math.floor(this.width / this.step);
        this.sy = Math.floor(this.height / this.step);
        this.metaFill = this.createRadialGradient(width, height, width, color1, color2);

        for (let i = 0; i < (this.sx + 2) * (this.sy + 2); i++) {
          this.grid[i] = new Vector2D(
            (i % (this.sx + 2)) * this.step,
            Math.floor(i / (this.sx + 2)) * this.step
          );
        }

        for (let i = 0; i < ballCount; i++) {
          this.balls[i] = new LavaBall(this);
        }
      }

      computeForce(x: number, y: number, index?: number): number {
        let force: number;
        const gridIndex = index || x + y * (this.sx + 2);

        if (x === 0 || y === 0 || x === this.sx || y === this.sy) {
          force = 0.6 * this.sign;
        } else {
          force = 0;
          const gridPoint = this.grid[gridIndex];

          for (const ball of this.balls) {
            force += ball.size * ball.size / (
              -2 * gridPoint.x * ball.pos.x -
              2 * gridPoint.y * ball.pos.y +
              ball.pos.magnitude +
              gridPoint.magnitude
            );
          }
          force *= this.sign;
        }

        this.grid[gridIndex].force = force;
        return force;
      }

      marchingSquares(coords: [number, number, number]): [number, number, number] | false {
        const [x, y, prevDirection] = coords;
        const gridIndex = x + y * (this.sx + 2);

        if (this.grid[gridIndex].computed === this.iter) {
          return false;
        }

        let caseValue = 0;
        for (let i = 0; i < 4; i++) {
          const neighborIndex = x + this.ix[i + 12] + (y + this.ix[i + 16]) * (this.sx + 2);
          let force = this.grid[neighborIndex].force;

          if ((force > 0 && this.sign < 0) || (force < 0 && this.sign > 0) || !force) {
            force = this.computeForce(x + this.ix[i + 12], y + this.ix[i + 16], neighborIndex);
          }

          if (Math.abs(force) > 1) {
            caseValue += Math.pow(2, i);
          }
        }

        if (caseValue === 15) {
          return [x, y - 1, 0];
        }

        let direction: number;
        if (caseValue === 5) {
          direction = prevDirection === 2 ? 3 : 1;
        } else if (caseValue === 10) {
          direction = prevDirection === 3 ? 0 : 2;
        } else {
          direction = this.mscases[caseValue];
          this.grid[gridIndex].computed = this.iter;
        }

        const interpolation = this.step / (
          Math.abs(
            Math.abs(
              this.grid[x + this.plx[4 * direction + 2] + (y + this.ply[4 * direction + 2]) * (this.sx + 2)].force
            ) - 1
          ) / Math.abs(
            Math.abs(
              this.grid[x + this.plx[4 * direction + 3] + (y + this.ply[4 * direction + 3]) * (this.sx + 2)].force
            ) - 1
          ) + 1
        );

        const ctx = canvas.ctx!;
        ctx.lineTo(
          this.grid[x + this.plx[4 * direction] + (y + this.ply[4 * direction]) * (this.sx + 2)].x + this.ix[direction] * interpolation,
          this.grid[x + this.plx[4 * direction + 1] + (y + this.ply[4 * direction + 1]) * (this.sx + 2)].y + this.ix[direction + 4] * interpolation
        );

        this.paint = true;
        return [x + this.ix[direction + 4], y + this.ix[direction + 8], direction];
      }

      renderMetaballs(): void {
        for (const ball of this.balls) {
          ball.move();
        }

        this.iter++;
        this.sign = -this.sign;
        this.paint = false;

        const ctx = canvas.ctx!;
        ctx.fillStyle = this.metaFill;
        ctx.beginPath();

        for (const ball of this.balls) {
          let coords: [number, number, number] | false = [
            Math.round(ball.pos.x / this.step),
            Math.round(ball.pos.y / this.step),
            0
          ];

          do {
            coords = this.marchingSquares(coords);
          } while (coords);

          if (this.paint) {
            ctx.fill();
            ctx.closePath();
            ctx.beginPath();
            this.paint = false;
          }
        }
      }

      private createRadialGradient(width: number, height: number, size: number, color1: string, color2: string): CanvasGradient {
        const ctx = canvas.ctx!;
        const gradient = ctx.createRadialGradient(width / 1, height / 1, 0, width / 1, height / 1, size);
        gradient.addColorStop(0, color1);
        gradient.addColorStop(1, color2);
        return gradient;
      }
    }

    if (document.getElementById("lamp-anim")) {
      const animate = (): void => {
        requestAnimationFrame(animate);
        const ctx = canvas.ctx!;
        ctx.clearRect(0, 0, canvas.canvasWidth, canvas.canvasHeight);
        metaballField.renderMetaballs();
      };

      canvas = screenModule.init("lamp-anim", null, true);
      canvas.resize();
      metaballField = new MetaballRenderer(canvas.canvasWidth, canvas.canvasHeight, 6, "#25CED1", "#ec407a");

      return { run: animate };
    }

    return { run: () => { } };
  })();

  lavaAnimation.run();
};

lavaLamp();

for (let i = 0; i < 200; i++) {
  const span = document.createElement("span");
  span.textContent = "HELLO";
  document.querySelector(".leftBar")?.appendChild(span);
  const span2 = document.createElement("span");
  span2.textContent = "HELLO";
  document.querySelector(".rightBar")?.appendChild(span2);
}
