import { sleep, initCanvas } from "./canvas.js";
import type { Coord } from "./canvas.js";

export type Box = {
    a: Coord; // NW
    b: Coord; // NE
    c: Coord; // SW
    d: Coord; // SE
    color: string;
};

// "shoelace formula"
function polygonArea(box: Box): number {
    const points: Coord[] = [box.a, box.b, box.d, box.c];
    let sum = 0;
    for (let i = 0; i < points.length; i++) {
        const [x1, y1] = points[i];
        const [x2, y2] = points[(i + 1) % points.length];
        sum += x1 * y2 - x2 * y1;
    }
    return Math.abs(sum) * 0.5;
}

function largestBox(boxes: Box[]) {
    let biggest = 0;
    let biggestIndex = 0;
    for (let i = 0; i < boxes.length; i++) {
        const area = polygonArea(boxes[i]);
        if (area > biggest) {
            biggest = area;
            biggestIndex = i;
        }
    }
    return biggestIndex;
}

function randomColor() {
    function r() {
        return Math.floor(Math.random() * 255);
    }
    return `rgb(${r()} ${r()} ${r()})`;
}

async function splits() {
    const canvas = initCanvas();
    if (!canvas) {
        console.error("Canvas failed to intialize");
        return;
    }
    const ctx = canvas.getContext("2d");
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";

    function drawBox(box: Box) {
        ctx.beginPath();
        ctx.fillStyle = box.color;
        ctx.moveTo(box.a[0], box.a[1]);
        ctx.lineTo(box.b[0], box.b[1]);
        ctx.lineTo(box.d[0], box.d[1]);
        ctx.lineTo(box.c[0], box.c[1]);
        ctx.fill();
    }

    async function boxes(width: number, height: number) {
        const initialBox: Box = {
            a: [0, 0],
            b: [width, 0],
            c: [0, height],
            d: [width, height],
            color: randomColor(),
        };
        const boxes: Box[] = [initialBox];

        async function splitBox(boxes: Box[], index: number) {
            const targetBox: Box = boxes[index];

            const height = Math.max(
                targetBox.c[1] - targetBox.a[1],
                targetBox.d[1] - targetBox.b[1],
            );
            const width = Math.max(
                targetBox.b[0] - targetBox.b[0],
                targetBox.d[0] - targetBox.c[0],
            );

            if (width > height) {
                let mult = Math.random() / 2 + 0.25;
                const breakpointN: Coord = [
                    mult * (targetBox.b[0] - targetBox.a[0]) + targetBox.a[0],
                    mult * (targetBox.b[1] - targetBox.a[1]) + targetBox.a[1],
                ];
                mult = Math.random() / 2 + 0.25;
                const breakpointS: Coord = [
                    mult * (targetBox.d[0] - targetBox.c[0]) + targetBox.c[0],
                    mult * (targetBox.d[1] - targetBox.c[1]) + targetBox.c[1],
                ];
                const newBox: Box = {
                    a: targetBox.a,
                    b: breakpointN,
                    c: targetBox.c,
                    d: breakpointS,
                    color: randomColor(),
                };
                targetBox.a = breakpointN;
                targetBox.c = breakpointS;
                boxes.push(newBox);
            } else {
                let mult = Math.random() / 2 + 0.25;
                const breakpointW: Coord = [
                    mult * (targetBox.c[0] - targetBox.a[0]) + targetBox.a[0],
                    mult * (targetBox.c[1] - targetBox.a[1]) + targetBox.a[1],
                ];
                mult = Math.random() / 2 + 0.25;
                const breakpointE: Coord = [
                    mult * (targetBox.d[0] - targetBox.b[0]) + targetBox.b[0],
                    mult * (targetBox.d[1] - targetBox.b[1]) + targetBox.b[1],
                ];
                const newBox: Box = {
                    a: targetBox.a,
                    b: targetBox.b,
                    c: breakpointW,
                    d: breakpointE,
                    color: randomColor(),
                };
                targetBox.a = breakpointW;
                targetBox.b = breakpointE;
                boxes.push(newBox);
            }
        }

        while (true) {
            ctx.clearRect(0, 0, width, height);
            boxes.forEach(drawBox);

            const largestIndex = largestBox(boxes);

            await splitBox(boxes, largestIndex);

            await sleep(250);
        }
    }
    boxes(canvas.width, canvas.height);
}

window.onload = async function () {
    await splits();
};
