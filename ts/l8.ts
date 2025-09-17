import { sleep, initCanvas } from "./canvas.js"
import type { Coord } from "./canvas.js"


async function l8() {

  const canvas = initCanvas();
  if (!canvas) {
    console.error("Canvas failed to intialize");
    return;
  }
  const ctx = canvas.getContext("2d");
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";



  while (true) {
    ctx.clearRect(0, 0, canvas.width, canvas.height);



    await sleep(50);
  }

}

window.onload = async function () {
  await l8();
};
