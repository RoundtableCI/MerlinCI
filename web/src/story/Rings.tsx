

import { useEffect, useRef, type MutableRefObject } from "react";

export type RingsFx = { p: number; t: number; cx: number; cy: number };

type Props = {
  /** written every frame by the story's render function */
  fxRef: MutableRefObject<RingsFx>;
  /** "r,g,b" of the ring strokes */
  ink?: string;
  /** "r,g,b" of the satellites */
  accent?: string;
  rings?: number;
  satellites?: number;
};

/** Ripples radiating from (cx, cy) with small glowing satellites riding them. Scroll and time push the rings outward. */
export function Rings({ fxRef, ink = "20,18,14", accent = "43,59,255", rings = 16, satellites = 7 }: Props) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;

    let W = 0;
    let H = 0;
    let raf = 0;

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      W = canvas.offsetWidth;
      H = canvas.offsetHeight;
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    const ro = new ResizeObserver(resize);
    ro.observe(canvas);

    const draw = () => {
      raf = 0;
      const { p, t, cx, cy } = fxRef.current;
      ctx.clearRect(0, 0, W, H);
      const x = cx * W;
      const y = cy * H;
      const maxR = Math.hypot(Math.max(x, W - x), Math.max(y, H - y)) * 1.02;

      for (let k = 0; k < rings; k++) {
        const u = (((k / rings + p * 1.6 + t * 0.018) % 1) + 1) % 1;
        ctx.beginPath();
        ctx.strokeStyle = `rgba(${ink},${Math.pow(1 - u, 1.7) * 0.2})`;
        ctx.lineWidth = 1;
        ctx.ellipse(x, y, u * maxR, u * maxR * 0.86, 0, 0, Math.PI * 2);
        ctx.stroke();
      }

      for (let s = 0; s < satellites; s++) {
        const u = (((s / satellites + p * 1.6 + t * 0.018) % 1) + 1) % 1;
        const r = u * maxR;
        const ang = t * (0.25 + s * 0.07) * (s % 2 ? 1 : -1) + s * 1.9 + p * 6;
        const a = Math.pow(1 - u, 1.2);
        ctx.beginPath();
        ctx.fillStyle = `rgba(${accent},${0.75 * a})`;
        ctx.shadowColor = `rgba(${accent},${0.6 * a})`;
        ctx.shadowBlur = 10;
        ctx.arc(x + Math.cos(ang) * r, y + Math.sin(ang) * r * 0.86, 2.6, 0, Math.PI * 2);
        ctx.fill();
        ctx.shadowBlur = 0;
      }
      if (visible) raf = requestAnimationFrame(draw);
    };

    // Only draw while the canvas is on screen.
    let visible = false;
    const io = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      if (visible && !raf) raf = requestAnimationFrame(draw);
    });
    io.observe(canvas);
    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      io.disconnect();
    };
  }, [fxRef, ink, accent, rings, satellites]);

  return <canvas ref={ref} className="ss-canvas" aria-hidden />;
}
