

import { useCallback, useEffect, useRef } from "react";
import { clamp01 } from "./story-math";

export type Frame = {
  /** smoothed scroll progress through the track, 0..1 */
  p: number;
  /** ambient seconds; stops advancing while paused, off-screen, or under reduced motion */
  t: number;
  /** smoothed pointer position, -1..1 on each axis */
  mouse: { x: number; y: number };
  /** stage size in CSS px */
  W: number;
  H: number;
};

type Options = {
  /** share of the remaining distance covered per frame; higher = snappier */
  smoothing?: number;
  mouseSmoothing?: number;
  /** enable ?p= and ?t= pins for screenshots */
  debug?: boolean;
};

/**
 * Drives a sticky-stage scroll story. Attach trackRef to the tall wrapper and stageRef to its sticky child.
 * `render` runs every animation frame while the track is on screen and should only write
 * transform/opacity/SVG attributes via refs.
 */
export function useScrollStory(
  render: (f: Frame) => void,
  { smoothing = 0.16, mouseSmoothing = 0.07, debug = import.meta.env.DEV }: Options = {},
) {
  const trackRef = useRef<HTMLDivElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);
  const renderRef = useRef(render);
  renderRef.current = render;
  const pausedRef = useRef(false);

  useEffect(() => {
    const track = trackRef.current;
    const stage = stageRef.current;
    if (!track || !stage) return;

    const qs = new URLSearchParams(window.location.search);
    const pinP = debug ? qs.get("p") : null;
    const pinT = debug ? qs.get("t") : null;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const mouseTarget = { x: 0, y: 0 };
    const mouse = { x: 0, y: 0 };
    let target = 0;
    let shown = 0;
    let ambient = 0;
    let last = performance.now();
    let raf = 0;

    const measure = () => {
      if (pinP !== null) {
        target = clamp01(parseFloat(pinP) || 0);
        return;
      }
      const r = track.getBoundingClientRect();
      const max = r.height - stage.offsetHeight;
      target = max > 0 ? clamp01(-r.top / max) : 0;
    };
    measure();
    shown = target;

    const tick = () => {
      const now = performance.now();
      if (!pausedRef.current && !reduced) ambient += Math.min(now - last, 100) / 1000;
      last = now;
      const d = target - shown;
      shown = reduced || Math.abs(d) < 0.0003 ? target : shown + d * smoothing;
      mouse.x += (mouseTarget.x - mouse.x) * mouseSmoothing;
      mouse.y += (mouseTarget.y - mouse.y) * mouseSmoothing;
      const t = pinT !== null ? parseFloat(pinT) || 0 : ambient;
      renderRef.current({ p: shown, t, mouse, W: stage.clientWidth, H: stage.clientHeight });
      raf = requestAnimationFrame(tick);
    };

    // Only run the loop while the track is on screen.
    const io = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting && !raf) {
        last = performance.now();
        measure();
        raf = requestAnimationFrame(tick);
      } else if (!entry.isIntersecting && raf) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
    });
    io.observe(track);

    const onMove = (e: PointerEvent) => {
      if (reduced) return;
      mouseTarget.x = (e.clientX / window.innerWidth) * 2 - 1;
      mouseTarget.y = (e.clientY / window.innerHeight) * 2 - 1;
    };
    window.addEventListener("scroll", measure, { passive: true });
    window.addEventListener("resize", measure);
    window.addEventListener("pointermove", onMove, { passive: true });
    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      window.removeEventListener("scroll", measure);
      window.removeEventListener("resize", measure);
      window.removeEventListener("pointermove", onMove);
    };
  }, [smoothing, mouseSmoothing, debug]);

  /** Scroll so the story sits at progress p. Swap window.scrollTo for lenis.scrollTo(top) if the site uses Lenis. */
  const scrollToProgress = useCallback((p: number) => {
    const track = trackRef.current;
    const stage = stageRef.current;
    if (!track || !stage) return;
    const r = track.getBoundingClientRect();
    const max = r.height - stage.offsetHeight;
    window.scrollTo({ top: window.scrollY + r.top + clamp01(p) * max, behavior: "smooth" });
  }, []);

  /** Freeze or resume ambient (time-driven) motion. Scroll-driven motion keeps working. WCAG 2.2.2 needs a control for this. */
  const setAmbientPaused = useCallback((paused: boolean) => {
    pausedRef.current = paused;
  }, []);

  return { trackRef, stageRef, scrollToProgress, setAmbientPaused };
}
