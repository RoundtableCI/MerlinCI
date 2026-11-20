// The MerlinCI scroll story: one brand's marketing, moving through the loop.
// One sticky stage, six scenes, six cards. The cards start as a loose stack, are dealt out as
// signals found in public, turn into a brief and its assets, queue for the brand's channels
// (one is stamped: it waits for a person), are filed as results, and end as the handful of
// text files a brand brings. Scroll position becomes one number p; everything is drawn from
// p and ambient time t. Engine: story-math.ts + useScrollStory.ts. Look: scroll-story.css.
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useScrollStory, type Frame } from './useScrollStory';
import { blendPose, clamp01, fadeIn, sceneCenters, sceneWeights, type Pose } from './story-math';
import { decide } from '../lib/policy';
import { getLenis } from '../lib/motion';
import { Arrow, REPO } from '../landing/shared';
import './scroll-story.css';

/* ---------- 1. Content ---------- */

const SCENES = ['Start', 'Discover', 'Produce', 'Distribute', 'Measure', 'Your brand'];

type Face = { kind: string; name: string; note: string };

// Each card shows four faces over the story: what was found, what was made from it,
// what is measured, and the file a brand supplies.
const CARDS: { tone: string; faces: [Face, Face, Face, Face] }[] = [
  { tone: 'ink', faces: [
    { kind: 'Demand', name: 'A question people keep asking', note: 'found in a public forum' },
    { kind: 'Decide', name: 'Campaign brief', note: 'audience, angle, what not to say' },
    { kind: 'Result', name: 'Traffic', note: 'tracked by campaign' },
    { kind: 'You bring', name: 'profile.yaml', note: 'your company and what may be claimed' },
  ] },
  { tone: '', faces: [
    { kind: 'Trend', name: 'A topic rising this week', note: 'with the pages that show it' },
    { kind: 'Produce', name: 'Article', note: 'every claim traced to a source' },
    { kind: 'Result', name: 'Signups', note: 'tracked by campaign' },
    { kind: 'You bring', name: 'audiences.yaml', note: 'who you think you are talking to' },
  ] },
  { tone: 'teal', faces: [
    { kind: 'Competitor', name: 'A rival changes its pitch', note: 'from what it published' },
    { kind: 'Produce', name: 'Social posts', note: 'written from the same brief' },
    { kind: 'North star', name: 'Customers activated', note: 'the number that counts' },
    { kind: 'You bring', name: 'policy.yaml', note: 'what may run without you' },
  ] },
  { tone: '', faces: [
    { kind: 'Conversation', name: 'A thread in your community', note: 'in the audience’s own words' },
    { kind: 'Produce', name: 'Email', note: 'for people who asked to hear' },
    { kind: 'Learn', name: 'Experiments', note: 'run and compared' },
    { kind: 'You bring', name: 'budget.yaml', note: 'what it may spend, and no more' },
  ] },
  { tone: '', faces: [
    { kind: 'Search', name: 'A phrase people type', note: 'and nobody answers well' },
    { kind: 'Produce', name: 'Graphics', note: 'in your brand’s look' },
    { kind: 'Learn', name: 'What worked', note: 'kept for next time' },
    { kind: 'You choose', name: 'Roles', note: 'researcher, writer, analyst and more' },
  ] },
  { tone: '', faces: [
    { kind: 'Gap', name: 'A subject no one covers', note: 'checked against what exists' },
    { kind: 'Produce', name: 'Video script', note: 'ready for a person to record' },
    { kind: 'Learn', name: 'The next cycle', note: 'starts from what was learned' },
    { kind: 'You choose', name: 'Workflows', note: 'the order the work happens in' },
  ] },
];
const N = CARDS.length;
const BRIEF = 0;   // the card that becomes the brief
const STAMPED = 1; // the article is the one stopped at the gate
const VERDICT = decide('publish.post'); // the real policy's answer

const HEADLINE = ['A marketing team', 'for your'];
const ROTATING = ['brand', 'startup', 'product', 'studio'];
const LEDE =
  'MerlinCI is open-source software that runs a brand’s marketing end to end: it finds what your audience is asking, plans the campaign, makes the content, prepares it for your channels and tracks what worked. You set the limits. It shows its work.';

const CAPTIONS: ({ title: ReactNode; body: string } | null)[] = [
  null,
  {
    title: 'It starts by listening',
    body: 'Demand, conversations, trends and competitor moves, found in permitted public sources. Each one is saved with the pages it came from, so nothing rests on a guess.',
  },
  {
    title: 'Then it makes the campaign',
    body: 'The best opportunity becomes a brief. The brief becomes articles, posts, email, graphics and scripts, each traced to its sources and checked against what your brand is allowed to say.',
  },
  {
    title: 'Nothing goes out without your rules',
    body: 'Publishing happens through official channels, and only inside the policy you wrote. Here an agent proposes to publish; your policy says a person approves first.',
  },
  {
    title: 'Judged on customers, not likes',
    body: 'Traffic and signups are tracked by campaign, but success is the number you choose: for our first customer, new customers who actually start using the product. What is learned feeds the next cycle.',
  },
  null,
];

const FINALE = {
  title: 'Any brand can run it.',
  body: 'Nothing about a company is written into the program. You describe your brand in a few text files and host it yourself. RoundtableCI is the first to run on it.',
};

/* ---------- 2. Timeline: hand-off windows between scenes (ascending, never overlapping) ---------- */

const TR: [number, number][] = [
  [0.07, 0.15],
  [0.25, 0.33],
  [0.43, 0.51],
  [0.61, 0.69],
  [0.82, 0.92],
];
const CENTERS = sceneCenters(TR);
const WORD_CYCLE = 2.6;
// which face a scene shows
const FACE_OF = [0, 0, 1, 1, 2, 3];

/* ---------- 3. Poses: where card i sits in scene k (fractions of the stage), and how it is turned ---------- */

type Sheet = Pose & { r: number };

function pose(k: number, i: number, f: Frame, narrow: boolean): Sheet {
  const { W, t } = f;
  const base = narrow ? 1 : Math.min(1, Math.max(0.66, W / 1440)); // cards shrink a little on small laptops
  const c = i - (N - 1) / 2; // -2.5 .. 2.5
  if (k === 0) {
    // a loose stack, breathing
    const bob = Math.sin(t * 0.7 + i * 1.3) * 0.006;
    return narrow
      ? { x: 0.5 + c * 0.03, y: 0.78 + c * 0.01 + bob, s: 0.9, o: 1, z: N - i, r: c * 5 }
      : { x: 0.8 + c * 0.02, y: 0.52 + c * 0.032 + bob, s: 1.2 * base, o: 1, z: N - i, r: c * 6 };
  }
  if (k === 1) {
    // dealt out: six things found
    if (narrow) return { x: i % 2 ? 0.74 : 0.26, y: 0.14 + Math.floor(i / 2) * 0.15, s: 0.9, o: 1, z: 0, r: (i % 2 ? 1 : -1) * 1.5 };
    return { x: 0.565 + (i % 3) * 0.165, y: i < 3 ? 0.36 : 0.66, s: 1.05 * base, o: 1, z: 0, r: (i % 2 ? 1 : -1) * 1.5 };
  }
  if (k === 2) {
    // the brief on top, its five assets fanned beneath it
    if (i === BRIEF) return narrow ? { x: 0.5, y: 0.16, s: 1.15, o: 1, z: 3, r: -2 } : { x: 0.72, y: 0.3, s: 1.45 * base, o: 1, z: 3, r: -2 };
    const j = i - 3; // -2 .. 2
    return narrow
      ? { x: 0.5 + j * 0.19, y: 0.4 + Math.abs(j) * 0.02, s: 0.62, o: 1, z: 1, r: j * 5 }
      : { x: 0.71 + j * 0.112, y: 0.7 + Math.abs(j) * 0.022, s: 0.74 * base, o: 1, z: 1, r: j * 5 };
  }
  if (k === 3) {
    // queued for the channels; the brief stays behind
    if (i === BRIEF) return narrow ? { x: 0.16, y: 0.1, s: 0.42, o: 0.5, z: 0, r: -4 } : { x: 0.5, y: 0.16, s: 0.5 * base, o: 0.5, z: 0, r: -4 };
    if (i === STAMPED) return narrow ? { x: 0.5, y: 0.24, s: 1.2, o: 1, z: 3, r: -3 } : { x: 0.7, y: 0.4, s: 1.6 * base, o: 1, z: 3, r: -3 };
    const j = i - 3.5; // -1.5 .. 1.5
    return narrow
      ? { x: 0.5 + j * 0.23, y: 0.5, s: 0.55, o: 0.9, z: 1, r: 0 }
      : { x: 0.7 + j * 0.12, y: 0.82, s: 0.62 * base, o: 0.9, z: 1, r: 0 };
  }
  if (k === 4) {
    // filed: one line each, top to bottom
    return narrow
      ? { x: 0.22, y: 0.085 + i * 0.082, s: 0.52, o: 1, z: 0, r: 0 }
      : { x: 0.6, y: 0.165 + i * 0.112, s: 0.66 * base, o: 1, z: 0, r: 0 };
  }
  // the files a brand brings, laid out in a row under the closing line
  return narrow
    ? { x: i % 2 ? 0.74 : 0.26, y: 0.55 + Math.floor(i / 2) * 0.125, s: 0.86, o: 1, z: 0, r: 0 }
    : { x: 0.5 + c * 0.152, y: 0.76, s: 0.98 * base, o: 1, z: 0, r: c * 1.2 };
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

  const cardRefs = useRef<(HTMLDivElement | null)[]>([]);
  const faceRefs = useRef<(HTMLDivElement | null)[][]>(CARDS.map(() => []));
  const sideRefs = useRef<(HTMLSpanElement | null)[]>([]);
  const heroRef = useRef<HTMLDivElement>(null);
  const line1Ref = useRef<HTMLSpanElement>(null);
  const line2Ref = useRef<HTMLSpanElement>(null);
  const finaleRef = useRef<HTMLDivElement>(null);
  const capRefs = useRef<(HTMLDivElement | null)[]>([]);
  const measureRefs = useRef<(HTMLSpanElement | null)[]>([]);
  const progressRef = useRef<HTMLElement>(null);
  const stampRef = useRef<HTMLDivElement>(null);
  const askRef = useRef<HTMLDivElement>(null);
  const rulesRef = useRef<HTMLDivElement>(null);

  const render = useCallback((f: Frame) => {
    const { W, H, p, mouse } = f;
    if (!W || !H) return;
    const narrow = W < 820;
    const w = sceneWeights(p, TR);
    const vis = w.map((x) => fadeIn(x));
    const last = w.length - 1;
    const cw = narrow ? 150 : 200;
    const ch = narrow ? 100 : 128;
    // how much of each face shows: the weights of the scenes that use it, added up
    const face = [0, 0, 0, 0];
    w.forEach((x, k) => { face[FACE_OF[k]] += x; });
    let stamped: Sheet | null = null;

    CARDS.forEach((_, i) => {
      let r = 0;
      const q = blendPose(w, (k) => {
        const s = pose(k, i, f, narrow);
        r += w[k] * s.r;
        return s;
      });
      const drift = w[0] * (N - i) * 0.5; // the stack leans toward the pointer
      const x = q.x * W + mouse.x * 10 * drift;
      const y = q.y * H + mouse.y * 6 * drift;
      const el = cardRefs.current[i];
      if (el) {
        el.style.transform = `translate(${x - cw / 2}px, ${y - ch / 2}px) rotate(${r}deg) scale(${q.s})`;
        el.style.opacity = String(q.o);
        el.style.zIndex = String(Math.round(50 + q.z * 10));
      }
      faceRefs.current[i].forEach((fe, j) => {
        if (fe) fe.style.opacity = String(fadeIn(face[j], 0.5));
      });
      const side = sideRefs.current[i];
      if (side) side.style.opacity = String(vis[4]);
      if (i === STAMPED) stamped = { ...q, x, y, r };
    });

    // scene 3: the proposal, then the stamp comes down on the card
    if (stamped) {
      const a = stamped as Sheet;
      if (askRef.current) {
        askRef.current.style.opacity = String(vis[3]);
        askRef.current.style.transform = `translate(calc(${a.x}px - 50%), ${a.y - (ch * a.s) / 2 - 40 - (1 - vis[3]) * 14}px)`;
      }
      if (stampRef.current) {
        const hit = clamp01((w[3] - 0.6) / 0.4); // lands late, so it reads as an answer
        stampRef.current.style.opacity = String(hit);
        stampRef.current.style.transform = `translate(calc(${a.x + cw * a.s * 0.16}px - 50%), calc(${a.y + ch * a.s * 0.2}px - 50%)) rotate(-9deg) scale(${1 + (1 - hit) * 1.4})`;
      }
    }

    if (rulesRef.current) rulesRef.current.style.transform = `translateY(${-p * 12}%)`;
    if (heroRef.current) {
      heroRef.current.style.opacity = String(vis[0]);
      heroRef.current.style.pointerEvents = vis[0] > 0.5 ? 'auto' : 'none';
    }
    const exit = 1 - w[0];
    if (line1Ref.current) line1Ref.current.style.transform = `translateX(${-exit * 160}px)`;
    if (line2Ref.current) line2Ref.current.style.transform = `translateX(${-exit * 320}px)`;
    if (finaleRef.current) {
      finaleRef.current.style.opacity = String(vis[last]);
      finaleRef.current.style.transform = `translateY(${(1 - vis[last]) * 34}px)`;
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

    const cyc = p > 0.004 ? wordRef.current : Math.floor(f.t / WORD_CYCLE) % ROTATING.length;
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
    <section className="ss" aria-label="How MerlinCI runs a brand's marketing">
      <a className="ss-skip" href="#record">Skip the animation</a>
      <div ref={trackRef} className="ss-track" style={{ '--ss-screens': 10 } as CSSProperties}>
        <div ref={stageRef} className="ss-stage">
          <div ref={rulesRef} className="ss-rules" aria-hidden>
            {Array.from({ length: 34 }, (_, i) => <i key={i} />)}
          </div>

          {CARDS.map((c, i) => (
            <div key={i} ref={(el) => { cardRefs.current[i] = el; }} className="ss-abs ss-card" data-tone={c.tone} aria-hidden>
              {c.faces.map((face, j) => (
                <div key={j} ref={(el) => { faceRefs.current[i][j] = el; }} className="ss-face" style={{ opacity: j === 0 ? 1 : 0 }}>
                  <span className="ss-card-top">{face.kind}</span>
                  <span className="ss-card-name">{face.name}</span>
                  {j !== 2 && <span className="ss-card-note">{face.note}</span>}
                </div>
              ))}
              <span ref={(el) => { sideRefs.current[i] = el; }} className="ss-amount">{c.faces[2].note}</span>
            </div>
          ))}

          <div ref={askRef} className="ss-abs ss-ask" style={{ opacity: 0 }} aria-hidden>
            an agent proposes <b>publish.post</b>
          </div>
          <div ref={stampRef} className="ss-abs ss-stamp" style={{ opacity: 0 }} aria-hidden>{VERDICT.headline}</div>

          <div ref={heroRef} className="ss-hero">
            <h1 className="ss-h1" aria-label={`${HEADLINE.join(' ')} ${ROTATING[word]}.`}>
              <span ref={line1Ref} className="ss-line" aria-hidden>
                <Chars text={HEADLINE[0]} />
              </span>
              <span ref={line2Ref} className="ss-line" aria-hidden>
                <Chars text={HEADLINE[1]} offset={headlineOffset} />
                <span className="ss-slot" style={{ width: widths[word] ? `${widths[word]}em` : undefined }}>
                  <span className="ss-words" style={{ transform: `translateY(${-word * 1.02}em)` }}>
                    {ROTATING.map((r) => <i key={r}>{r}</i>)}
                  </span>
                </span>
                <span className="ss-ch" style={{ '--i': headlineOffset + HEADLINE[1].length } as CSSProperties}>.</span>
              </span>
            </h1>
            <p className="ss-lede">{LEDE}</p>
            <div className="ss-actions">
              <Link className="button" to="/app">Open the console <Arrow /></Link>
              <a className="link" href={`${REPO}/blob/main/docs/project-guide.md`}>Read the project guide</a>
            </div>
          </div>
          <div className="ss-measure-box" aria-hidden>
            {ROTATING.map((r, i) => (
              <span key={r} ref={(el) => { measureRefs.current[i] = el; }} className="ss-measure">{r}</span>
            ))}
          </div>

          <div ref={finaleRef} className="ss-hero ss-finale" style={{ opacity: 0 }}>
            <h2 className="ss-h1">{FINALE.title}</h2>
            <p className="ss-lede">{FINALE.body}</p>
            <div className="ss-actions">
              <Link className="button ink" to="/app">Open the console <Arrow /></Link>
            </div>
          </div>

          {CAPTIONS.map((c, k) =>
            c ? (
              <div key={k} ref={(el) => { capRefs.current[k] = el; }} className="ss-cap">
                <h2>{c.title}</h2>
                <p>{c.body}</p>
              </div>
            ) : null,
          )}

          <nav className="ss-rail" aria-label="Story progress">
            {SCENES.map((name, k) => (
              <button key={name} type="button" onClick={() => scrollToScene(k)} aria-current={active === k ? 'true' : undefined} aria-label={name}>
                <span>{name}</span>
              </button>
            ))}
          </nav>
          <button type="button" className="ss-pause" onClick={() => onPause(!paused)} aria-pressed={paused}>
            {paused ? 'Play motion' : 'Pause motion'}
          </button>
          <div className="ss-progress" aria-hidden><s ref={progressRef} /></div>
        </div>
      </div>
    </section>
  );
}
