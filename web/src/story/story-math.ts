export type Pose = { x: number; y: number; s: number; o: number; z: number };

export const clamp01 = (x: number) => Math.min(1, Math.max(0, x));
export const smoothstep = (t: number) => t * t * (3 - 2 * t);
export const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

/** 0 before a, 1 after b, smoothstep-eased in between. */
export const win = (p: number, a: number, b: number) => smoothstep(clamp01((p - a) / (b - a)));

/** Text visibility: only rises once a scene clearly dominates, so captions never overlap half-faded. */
export const fadeIn = (w: number, threshold = 0.35) => clamp01((w - threshold) / (1 - threshold));

/** n-1 evenly spaced, non-overlapping hand-off windows for n scenes. `width` is the share of each segment spent transitioning. */
export function evenTransitions(n: number, width = 0.5): [number, number][] {
  const seg = 1 / n;
  return Array.from({ length: n - 1 }, (_, k) => {
    const c = (k + 1) * seg;
    return [c - (seg * width) / 2, c + (seg * width) / 2] as [number, number];
  });
}

/** Weight of each scene at progress p. With ascending, non-overlapping windows the weights sum to 1. */
export function sceneWeights(p: number, transitions: [number, number][]): number[] {
  const tr = transitions.map(([a, b]) => win(p, a, b));
  const n = tr.length + 1;
  return Array.from({ length: n }, (_, k) => (k === 0 ? 1 : tr[k - 1]) * (k === n - 1 ? 1 : 1 - tr[k]));
}

/** Progress value at the middle of each scene's hold, for jumping to a scene. */
export function sceneCenters(transitions: [number, number][]): number[] {
  const n = transitions.length + 1;
  return Array.from({ length: n }, (_, k) => {
    const start = k === 0 ? 0 : transitions[k - 1][1];
    const end = k === n - 1 ? 1 : transitions[k][0];
    return (start + end) / 2;
  });
}

/** Weighted average of every scene's pose. Scenes with negligible weight are skipped. */
export function blendPose(weights: number[], poseFor: (scene: number) => Pose): Pose {
  const out: Pose = { x: 0, y: 0, s: 0, o: 0, z: 0 };
  weights.forEach((w, k) => {
    if (w < 0.0005) return;
    const q = poseFor(k);
    out.x += w * q.x;
    out.y += w * q.y;
    out.s += w * q.s;
    out.o += w * q.o;
    out.z += w * q.z;
  });
  return out;
}
