import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import SplitText from '../components/reactbits/SplitText';
import Magnet from '../components/reactbits/Magnet';
import { decide } from '../lib/policy';
import { gsap, reducedMotion } from '../lib/motion';
import { Arrow, REPO } from './shared';

// What a role might propose, and what the real policy answers. The verdicts are computed
// by lib/policy.ts from the rules in policy.yaml; nothing here is typed in by hand.
const PROPOSALS: { role: string; action: string; what: string }[] = [
  { role: 'Market Researcher', action: 'research.search', what: 'search the public web' },
  { role: 'Distribution Manager', action: 'publish.post', what: 'publish the piece' },
  { role: 'Content Writer', action: 'draft.article', what: 'draft an article' },
  { role: 'Content Writer', action: 'budget.increase', what: 'raise its own cap' },
  { role: 'Campaign Strategist', action: 'campaign.plan', what: 'plan the campaign' },
  { role: 'Distribution Manager', action: 'spend.ads', what: 'buy an ad' },
  { role: 'Market Researcher', action: 'account.login', what: 'sign in to a site' },
];

/** The gap in the headline: a model proposes, MerlinCI answers, on a loop. */
export function Strip({ paused }: { paused: boolean }) {
  const [i, setI] = useState(0);
  const [prev, setPrev] = useState<number | null>(null);
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (paused || reducedMotion()) return;
    const id = window.setInterval(() => {
      setI((n) => {
        setPrev(n);
        return (n + 1) % PROPOSALS.length;
      });
    }, 2600);
    return () => window.clearInterval(id);
  }, [paused]);

  // The signature move: the strip opens from its left edge, once.
  useEffect(() => {
    if (!ref.current || reducedMotion()) return;
    const tween = gsap.fromTo(
      ref.current,
      { clipPath: 'inset(0 100% 0 0 round 14px)' },
      { clipPath: 'inset(0 0% 0 0 round 14px)', duration: 1.1, ease: 'power3.inOut', delay: 0.35, clearProps: 'clipPath' },
    );
    return () => { tween.kill(); };
  }, []);

  const row = (n: number, cls: string) => {
    const p = PROPOSALS[n];
    const v = decide(p.action);
    return (
      <span className={`strip-row ${cls}`} key={`${n}-${cls}`}>
        <span className="strip-ask">
          <span>{p.role} proposes</span>
          <b>{p.action}</b>
        </span>
        <span className="strip-verdict" data-outcome={v.outcome}>{paused ? 'Paused' : v.headline}</span>
      </span>
    );
  };

  return (
    <span className="strip" data-ui ref={ref} aria-hidden="true">
      {prev !== null && prev !== i && row(prev, 'leave')}
      {row(i, prev === null ? '' : 'enter')}
    </span>
  );
}

export function Hero({ paused }: { paused: boolean }) {
  return (
    <header className="hero wrap">
      <h1 className="display" aria-label="Marketing that shows its work.">
        <span className="hero-line" aria-hidden="true">
          <SplitText tag="span" text="Marketing that" splitType="chars" trigger="load" textAlign="left" delay={22} duration={1} />
          <Strip paused={paused} />
        </span>
        <span className="hero-line" aria-hidden="true">
          <SplitText tag="span" text="shows its work." splitType="chars" trigger="load" textAlign="left" delay={22} duration={1} />
        </span>
      </h1>
      <div className="hero-under" data-rise-group>
        <p className="lead" data-rise>
          MerlinCI is an open-source marketing system that runs campaigns on its own, inside limits you set.
          Every action is checked against a policy, costed and recorded.
        </p>
        <div className="hero-actions" data-rise>
          <a className="link" href={`${REPO}/blob/main/docs/project-guide.md`}>Read the project guide</a>
          <Magnet padding={48} magnetStrength={5}>
            <Link className="button" to="/app">Open the console <Arrow /></Link>
          </Magnet>
        </div>
      </div>
    </header>
  );
}
