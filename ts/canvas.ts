export type Coord = [number, number];

export async function sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

export function initCanvas(): HTMLCanvasElement | null {
    const canvas: HTMLCanvasElement | null = document.getElementById(
        "canvas",
    ) as HTMLCanvasElement;
    if (!canvas) {
        console.error("Canvas failed to intialize");
        return;
    }

    function resizeCanvas() {
        canvas.width = window.innerWidth;
        canvas.height = window.innerHeight;
        console.log(`Canvas printing at ${canvas.width}w ${canvas.height}h`);
    }

    resizeCanvas();

    return canvas;
}
