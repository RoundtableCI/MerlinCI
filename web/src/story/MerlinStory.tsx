// The MerlinCI scroll story: one sticky stage, five scenes, six actors (the roles of a campaign).
// Scroll position becomes one number p; every element is drawn from p and ambient time t.
// Engine: story-math.ts + useScrollStory.ts. Look: scroll-story.css, mapped to tokens.css.
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useScrollStory, type Frame } from './useScrollStory';
import { blendPose, clamp01, fadeIn, lerp, sceneCenters, sceneWeights, type Pose } from './story-math';
import { Rings, type RingsFx } from './Rings';
import { ROLES, WORKFLOW } from '../data/sample';
import { getBudget } from '../data/api';
import { decide } from '../lib/policy';
import { money, roleLabel } from '../lib/format';
import { getLenis } from '../lib/motion';
import './scroll-story.css';

/* ---------- 1. Content ---------- */

const SCENES = ['Intro', 'The roles', 'The gate', 'The ledger', 'The record'];

// The actors are the six steps of the first-release workflow, with their real limits.
const ACTORS = WORKFLOW.map((w) => {
  const role = ROLES.find((r) => r.name === w.role);
  const name = roleLabel(w.role ?? w.evaluator);
  return {
    id: w.id,
    name,
    initial: name.split(' ').map((s) => s[0]).join(''),
    calls: role?.maxSteps ?? 1,
    meter: role ? `${role.maxSteps} calls · ${money(role.maxCostPerTask).replace('.00', '')}` : 'pass or stop',
  };
});
const MAX_CALLS = Math.max(...ACTORS.map((a) => a.calls));
const ASKER = 2; // the Content Writer is the one who asks in scene 2
const VERDICT = decide('budget.increase');
const DAY = getBudget().limits[0];

const HEADLINE = ['Marketing that', 'shows its'];
const ROTATING = ['work', 'sources', 'costs', 'rules'];
const LEDE =
  'MerlinCI is an open-source marketing system that runs campaigns on its own, inside limits you set. Every action is checked against a policy, costed and recorded.';

const CAPTIONS: ({ tag: string; title: ReactNode; body: string } | null)[] = [
  null,
  {
    tag: 'I · The roles',
    title: <>Six roles, <em>one campaign.</em></>,
    body: 'Researcher, strategist, writer, guardian, distribution, analyst. Each is a text file with its own tools, its own spending cap and its own limit on model calls.',
  },
  {
    tag: 'II · The gate',
    title: <>The model proposes. <em>MerlinCI decides.</em></>,
    body: 'Every tool call meets the policy before it runs. Here the writer asks to raise its own budget, and that always waits for a person.',
  },
  {
    tag: 'III · The ledger',
    title: <>Capped before <em>it is spent.</em></>,
    body: 'The worst-case cost of a call is reserved first. If it does not fit under the day’s limit, the call is never made.',
  },
  null,
];

const FINALE = {
  title: <>Then it <em>shows its work.</em></>,
  body: 'Every run leaves a record nobody can edit: what was proposed, what was allowed, what it cost and which pages it rests on.',
  cta: 'Open the console',
};

/* ---------- 2. Timeline: hand-off windows between scenes (ascending, never overlapping) ---------- */

const TR: [number, number][] = [
  [0.1, 0.2],
  [0.32, 0.42],
  [0.54, 0.64],
  [0.78, 0.9],
];
const CENTERS = sceneCenters(TR);
const WORD_CYCLE = 2.6;

/* ---------- 3. Poses: where actor i sits in scene k, as fractions of the stage ---------- */

function pose(k: number, i: number, n: number, f: Frame, narrow: boolean): Pose {
  const { W, H, p, t } = f;
  if (k === 0 || k === SCENES.length - 1) {
    // orbit around the headline
    const ang = ((-90 + (i * 360) / n + t * 9 + p * 360) * Math.PI) / 180;
    const ry = narrow ? 0.2 : 0.36;
    const rx = Math.min(W * 0.45, ry * H * (narrow ? 1.1 : 2.1)) / W;
    const sin = Math.sin(ang);
    return { x: 0.5 + rx * Math.cos(ang), y: (narrow ? 0.72 : 0.53) + ry * sin, s: 0.86 + 0.18 * ((sin + 1) / 2), o: 0.75 + 0.25 * ((sin + 1) / 2), z: sin };
  }
  const x0 = narrow ? 0.14 : 0.46;
  const x1 = narrow ? 0.86 : 0.93;
  const rowX = lerp(x0, x1, i / (n - 1));
  if (k === 1) return { x: rowX, y: narrow ? 0.3 : 0.48, s: 1, o: 1, z: 0 };
  if (k === 2) {
    // spotlight: the asker steps forward, the rest wait along the bottom
    return i === ASKER
      ? { x: narrow ? 0.5 : 0.6, y: narrow ? 0.22 : 0.4, s: 1.7, o: 1, z: 1 }
      : { x: rowX, y: narrow ? 0.5 : 0.86, s: 0.5, o: 0.45, z: 0 };
  }
  // the ledger: everyone around one table, the day's budget in the middle
  const ang = ((-90 + (i * 360) / n + t * 5) * Math.PI) / 180;
  const cx = narrow ? 0.5 : 0.7;
  const cy = narrow ? 0.3 : 0.5;
  const r = narrow ? 0.19 : 0.27;
  return { x: cx + ((r * H) / W) * Math.cos(ang) * (narrow ? 1 : 1.25), y: cy + r * Math.sin(ang), s: 0.8, o: 1, z: Math.sin(ang) };
}

/* ---------- 4. Component ---------- */

function Chars({ text, offset = 0 }: { text: string; offset?: number }) {
  return (
    <>
      {text.split('').map((c, i) => (
        <span key={i} className="ss-ch" style={{ '--i': i + offset } as CSSProperties}>
          {c === ' ' ? ' ' : c}
        </span>
      ))}
    </>
  );
}

export function MerlinStory({ paused, onPause }: { paused: boolean; onPause: (p: boolean) => void }) {
  const [active, setActive] = useState(0);
  const [word, setWord] = useState(0);
  const [widths, setWidths] = useState<number[]>([]);
  const activeRef = useRef(0);
  const wordRef = useRef(0);
  const fxRef = useRef<RingsFx>({ p: 0, t: 0, cx: 0.5, cy: 0.5 });

  const actorRefs = useRef<(HTMLDivElement | null)[]>([]);
  const labelRefs = useRef<(HTMLDivElement | null)[]>([]);
  const fillRefs = useRef<(HTMLElement | null)[]>([]);
  const winRef = useRef<HTMLDivElement>(null);
  const heroRef = useRef<HTMLDivElement>(null);
  const line1Ref = useRef<HTMLSpanElement>(null);
  const line2Ref = useRef<HTMLSpanElement>(null);
  const finaleRef = useRef<HTMLDivElement>(null);
  const capRefs = useRef<(HTMLDivElement | null)[]>([]);
  const measureRefs = useRef<(HTMLSpanElement | null)[]>([]);
  const progressRef = useRef<HTMLElement>(null);
  const verdictRef = useRef<HTMLDivElement>(null);
  const ledgerRef = useRef<HTMLDivElement>(null);
  const ledgerFillRef = useRef<HTMLElement>(null);
  const hintRef = useRef<HTMLDivElement>(null);

  const render = useCallback((f: Frame) => {
    const { W, H, p, t, mouse } = f;
    if (!W || !H) return;
    const narrow = W < 820;
    const w = sceneWeights(p, TR);
    const vis = w.map((x) => fadeIn(x));
    const last = w.length - 1;
    const heroish = w[0] + w[last];
    const D = Math.max(narrow ? 44 : 54, Math.min(narrow ? 58 : 88, H * 0.092));

    ACTORS.forEach((_, i) => {
      const q = blendPose(w, (k) => pose(k, i, ACTORS.length, f, narrow));
      const x = q.x * W + mouse.x * 16 * heroish * q.s;
      const y = q.y * H + mouse.y * 10 * heroish * q.s;
      const el = actorRefs.current[i];
      if (el) {
        el.style.width = `${D}px`;
        el.style.height = `${D}px`;
        el.style.transform = `translate(${x - D / 2}px, ${y - D / 2}px) scale(${q.s})`;
        el.style.opacity = String(q.o);
        el.style.zIndex = String(Math.round(50 + q.z * 30 + (i === ASKER ? 100 * w[2] : 0)));
      }
      const label = labelRefs.current[i];
      if (label) label.style.opacity = String(vis[1]);
      const fill = fillRefs.current[i];
      if (fill) fill.style.transform = `scaleX(${vis[1]})`;
    });
    if (winRef.current) winRef.current.style.opacity = String(vis[2]);

    // scene 2: the verdict card beside the asker
    if (verdictRef.current) {
      const vx = (narrow ? 0.5 : 0.6) * W;
      const vy = (narrow ? 0.22 : 0.4) * H + D * 1.25;
      verdictRef.current.style.opacity = String(vis[2]);
      verdictRef.current.style.transform = `translate(calc(${vx}px - 50%), ${vy + (1 - vis[2]) * 24}px)`;
    }
    // scene 3: the day's budget in the middle of the table
    if (ledgerRef.current) {
      const lx = (narrow ? 0.5 : 0.7) * W;
      const ly = (narrow ? 0.3 : 0.5) * H;
      ledgerRef.current.style.opacity = String(vis[3]);
      ledgerRef.current.style.transform = `translate(calc(${lx}px - 50%), calc(${ly}px - 50%)) scale(${0.92 + 0.08 * vis[3]})`;
    }
    if (ledgerFillRef.current) ledgerFillRef.current.style.transform = `scaleX(${vis[3] * (DAY.spent / DAY.limit)})`;

    if (heroRef.current) {
      heroRef.current.style.opacity = String(vis[0]);
      heroRef.current.style.transform = `translate(-50%, ${(1 - vis[0]) * -30}px)`;
      heroRef.current.style.pointerEvents = vis[0] > 0.5 ? 'auto' : 'none';
    }
    const exit = 1 - w[0];
    if (line1Ref.current) line1Ref.current.style.transform = `translateX(${-exit * 180}px)`;
    if (line2Ref.current) line2Ref.current.style.transform = `translateX(${exit * 180}px)`;
    if (hintRef.current) hintRef.current.style.opacity = String(clamp01(1 - p * 40));
    if (finaleRef.current) {
      finaleRef.current.style.opacity = String(vis[last]);
      finaleRef.current.style.transform = `translate(-50%, ${(1 - vis[last]) * 34}px)`;
      finaleRef.current.style.pointerEvents = vis[last] > 0.5 ? 'auto' : 'none';
    }
    CAPTIONS.forEach((c, k) => {
      const el = capRefs.current[k];
      if (!c || !el) return;
      const dy = (1 - vis[k]) * 28;
      el.style.opacity = String(vis[k]);
      el.style.transform = narrow ? `translateY(${dy}px)` : `translateY(calc(-50% + ${dy}px))`;
      el.style.pointerEvents = vis[k] > 0.5 ? 'auto' : 'none';
    });

    if (progressRef.current) progressRef.current.style.transform = `scaleX(${p})`;
    // the rings follow whatever the scene is about: the asker, then the table
    fxRef.current = {
      p, t,
      cx: 0.5 + w[2] * ((narrow ? 0.5 : 0.6) - 0.5) + w[3] * ((narrow ? 0.5 : 0.7) - 0.5),
      cy: 0.5 + w[2] * ((narrow ? 0.22 : 0.4) - 0.5) + w[3] * ((narrow ? 0.3 : 0.5) - 0.5),
    };

    const cyc = p > 0.004 ? wordRef.current : Math.floor(t / WORD_CYCLE) % ROTATING.length;
    if (cyc !== wordRef.current) {
      wordRef.current = cyc;
      setWord(cyc);
    }
    let act = 0;
    for (let k = 1; k < w.length; k++) if (w[k] > w[act]) act = k;
    if (act !== activeRef.current) {
      activeRef.current = act;
      setActive(act);
    }
  }, []);

  // Lenis already smooths the scroll, so the engine's own smoothing is kept light.
  const { trackRef, stageRef, setAmbientPaused } = useScrollStory(render, { smoothing: 0.3 });
  useEffect(() => setAmbientPaused(paused), [paused, setAmbientPaused]);

  const scrollToScene = (k: number) => {
    const track = trackRef.current;
    const stage = stageRef.current;
    if (!track || !stage) return;
    const r = track.getBoundingClientRect();
    const top = window.scrollY + r.top + CENTERS[k] * (r.height - stage.offsetHeight);
    const lenis = getLenis();
    if (lenis) lenis.scrollTo(top, { duration: 1.4 });
    else window.scrollTo({ top, behavior: 'smooth' });
  };

  // Rotating-word widths in em; re-measured once the web fonts have loaded.
  useLayoutEffect(() => {
    const measure = () => {
      const ws = measureRefs.current.map((el) => (el ? el.offsetWidth / (parseFloat(getComputedStyle(el).fontSize) || 16) : 0));
      if (ws.length && ws.every((x) => x > 0)) setWidths(ws);
    };
    measure();
    document.fonts?.ready.then(measure);
  }, []);

  const headlineOffset = HEADLINE[0].length;

  return (
    <section className={paused ? 'ss ss-paused' : 'ss'} aria-label="How MerlinCI works">
      <a className="ss-skip" href="#record">Skip the animation</a>
      <div ref={trackRef} className="ss-track" style={{ '--ss-screens': 8 } as CSSProperties}>
        <div ref={stageRef} className="ss-stage">
          <div className="ss-blob ss-b1" aria-hidden />
          <div className="ss-blob ss-b2" aria-hidden />
          <div className="ss-blob ss-b3" aria-hidden />
          <Rings fxRef={fxRef} ink="10,10,10" accent="37,111,130" />

          {ACTORS.map((a, i) => (
            <div key={a.id} ref={(el) => { actorRefs.current[i] = el; }} className="ss-abs ss-disc" role="img" aria-label={a.name}>
              {i === ASKER && <div ref={winRef} className="ss-win" />}
              <span className="ss-initial">{a.initial}</span>
              <div ref={(el) => { labelRefs.current[i] = el; }} className="ss-label">
                <b>{a.name}</b>
                <div className="ss-meter">
                  <i><s ref={(el) => { fillRefs.current[i] = el; }} style={{ width: `${(a.calls / MAX_CALLS) * 100}%` }} /></i>
                  {a.meter}
                </div>
              </div>
            </div>
          ))}

          <div ref={verdictRef} className="ss-abs ss-verdict" style={{ opacity: 0 }} aria-hidden>
            <span className="ss-mono">Content Writer proposes</span>
            <b>budget.increase</b>
            <span className="ss-verdict-out">{VERDICT.headline}</span>
          </div>

          <div ref={ledgerRef} className="ss-abs ss-ledger" style={{ opacity: 0 }} aria-hidden>
            <span className="ss-mono">AI models, today</span>
            <b>{money(DAY.spent)}</b>
            <i><s ref={ledgerFillRef} /></i>
            <span className="ss-ledger-of">of {money(DAY.limit)} · sample day</span>
          </div>

          <div ref={heroRef} className="ss-hero">
            <h1 className="ss-h1" aria-label={`${HEADLINE.join(' ')} ${ROTATING[word]}.`}>
              <span ref={line1Ref} className="ss-line" aria-hidden>
                <Chars text={HEADLINE[0]} />
              </span>
              <span ref={line2Ref} className="ss-line" aria-hidden>
                <Chars text={HEADLINE[1]} offset={headlineOffset} />
                <span className="ss-slot" style={{ width: widths[word] ? `${widths[word]}em` : undefined }}>
                  <span className="ss-words" style={{ transform: `translateY(${-word * 1.12}em)` }}>
                    {ROTATING.map((r) => <i key={r}>{r}</i>)}
                  </span>
                </span>
                <span className="ss-ch" style={{ '--i': headlineOffset + HEADLINE[1].length } as CSSProperties}>.</span>
              </span>
            </h1>
            <p className="ss-lede">{LEDE}</p>
            <Link className="ss-cta" to="/app">Open the console <span aria-hidden>→</span></Link>
          </div>
          <div className="ss-measure-box" aria-hidden>
            {ROTATING.map((r, i) => (
              <span key={r} ref={(el) => { measureRefs.current[i] = el; }} className="ss-measure">{r}</span>
            ))}
          </div>

          <div ref={finaleRef} className="ss-hero ss-finale" style={{ opacity: 0 }}>
            <h2 className="ss-h1">{FINALE.title}</h2>
            <p className="ss-lede">{FINALE.body}</p>
            <Link className="ss-cta" to="/app">{FINALE.cta} <span aria-hidden>→</span></Link>
          </div>

          {CAPTIONS.map((c, k) =>
            c ? (
              <div key={c.tag} ref={(el) => { capRefs.current[k] = el; }} className="ss-cap">
                <span className="ss-mono">{c.tag}</span>
                <h2>{c.title}</h2>
                <p>{c.body}</p>
              </div>
            ) : null,
          )}

          <nav className="ss-rail" aria-label="Story progress">
            {SCENES.map((name, k) => (
              <button key={name} type="button" onClick={() => scrollToScene(k)} aria-current={active === k} aria-label={name}>
                <span>{name}</span>
                <i />
              </button>
            ))}
          </nav>

          <div ref={hintRef} className="ss-hint ss-mono" aria-hidden>Scroll to watch a campaign run</div>
          <button type="button" className="ss-pause" onClick={() => onPause(!paused)} aria-pressed={paused}>
            {paused ? 'Play motion' : 'Pause motion'}
          </button>
          <div className="ss-progress" aria-hidden><s ref={progressRef} /></div>
        </div>
      </div>
    </section>
  );
}
