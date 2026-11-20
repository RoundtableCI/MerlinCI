import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import SplitText from '../components/reactbits/SplitText';
import ScrollReveal from '../components/reactbits/ScrollReveal';
import CountUp from '../components/reactbits/CountUp';
import Magnet from '../components/reactbits/Magnet';
import { Wordmark } from '../components/Mark';
import { EVENTS, ROLES, WORKFLOW } from '../data/sample';
import { getBudget } from '../data/api';
import { FLOORS, RULES, decide, matchAction } from '../lib/policy';
import { clock, money, percent, roleLabel } from '../lib/format';
import { ScrollTrigger, gsap, reducedMotion } from '../lib/motion';
import { Arrow, REPO } from './shared';

const Heading = ({ text, className = 'head' }: { text: string; className?: string }) => (
  <SplitText tag="h2" className={className} text={text} splitType="words" textAlign="left" delay={55} duration={1} rootMargin="-60px" />
);

// ---------- 1. the record ----------

export function Record() {
  const events = EVENTS.task_d3a8e6f41b90;
  return (
    <section className="section wrap" id="record">
      <div className="cols" data-rise-group>
        <div className="record-copy">
          <Heading text="Every run keeps a record" />
          <p data-rise>
            Each tool call is written down before it runs and again when it finishes, and the database
            refuses to let anyone change it. On the right is one run from the sample console: a writer,
            two rejected drafts, and a request it could not grant itself.
          </p>
          <dl className="facts" data-rise>
            <div><dt>Safety rules, each enforced in code</dt><dd><CountUp to={15} duration={1.2} /></dd></div>
            <div><dt>Actions that always need a person</dt><dd><CountUp to={FLOORS.length} duration={1.2} /></dd></div>
            <div><dt>Daily limit for AI models, in rupees</dt><dd><CountUp to={150} duration={1.4} /></dd></div>
            <div><dt>Roles, each one a text file</dt><dd><CountUp to={ROLES.length} duration={1.2} /></dd></div>
          </dl>
        </div>
        <figure className="record-shot ui" data-ui data-drift>
          <div className="ui-head">
            <strong>Run record</strong>
            <span>Content Writer · 19 Nov</span>
          </div>
          <ol className="ui-body ui-record">
            {events.map((e) => (
              <li key={e.at + e.type} data-tone={e.tone}>
                <time>{clock(e.at)}</time>
                <strong>{e.type}</strong>
                <p>{e.detail}</p>
              </li>
            ))}
          </ol>
        </figure>
      </div>
    </section>
  );
}

// ---------- 2. one campaign as one bar ----------
// Each segment is as wide as the most model calls its role may make (configs/roles).

const NOTES: Record<string, string> = {
  'find-opportunity': 'First, the researcher searches and reads public pages. A search result is not evidence; only a page it fetched is.',
  'write-brief': 'Then the strategist turns the research into a brief: audience, angle, key points, and what not to say.',
  'create-asset': 'Next, the writer drafts the piece. Every factual statement has to come from a saved source or the approved profile.',
  'brand-check': 'Brand Guardian reads it again, on its own. If it rejects, nothing after it runs.',
  'distribute': 'The distribution role prepares a package: final copy, short versions, tracking tags. A person publishes.',
  'record-results': 'Last, the analyst records what was made and how it will be measured. It reports no number it does not have.',
};

const SEGMENTS = WORKFLOW.map((w, i) => {
  const role = ROLES.find((r) => r.name === w.role);
  return {
    ...w,
    tone: i,
    weight: role?.maxSteps ?? 1,
    calls: role ? `${role.maxSteps} calls` : '1 check',
    cap: role ? `up to ${money(role.maxCostPerTask).replace('.00', '')}` : 'pass or stop',
    who: roleLabel(w.role ?? w.evaluator),
    note: NOTES[w.id],
  };
});

export function CampaignBar() {
  const scene = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = scene.current;
    if (!el || reducedMotion()) return;
    const mm = gsap.matchMedia();
    mm.add('(min-width: 901px)', () => {
      const segs = el.querySelectorAll('.bar .bar-seg');
      const notes = el.querySelectorAll('.bar-note');
      const tl = gsap.timeline({
        scrollTrigger: { trigger: el, start: 'center center', end: '+=140%', pin: true, scrub: 0.6, anticipatePin: 1 },
      });
      segs.forEach((seg, i) => {
        tl.fromTo(seg, { scaleX: 0 }, { scaleX: 1, duration: 1, ease: 'power2.out' }, i * 0.8)
          .fromTo(seg.children, { opacity: 0 }, { opacity: 1, duration: 0.4 }, i * 0.8 + 0.5)
          .fromTo(notes[i], { opacity: 0, y: 16 }, { opacity: 1, y: 0, duration: 0.7 }, i * 0.8 + 0.3);
      });
    });
    return () => mm.revert();
  }, []);

  return (
    <section className="section wrap" id="campaign">
      <div ref={scene}>
        <div className="bar-head" data-rise-group>
          <Heading text="One campaign, six steps" />
          <p data-rise>
            Each step is a task of its own, with its own spending cap and its own limit on model calls.
            The widths below are those limits.
          </p>
        </div>
        <ol className="bar" data-ui>
          {SEGMENTS.map((s) => (
            <li key={s.id} className="bar-col" style={{ flexGrow: s.weight }}>
              <div className="bar-seg" data-tone={s.tone}>
                <b>{s.calls}</b>
                <span>{s.cap}</span>
              </div>
              <div className="bar-note">
                <strong>{s.who}</strong>
                <p>{s.note}</p>
              </div>
            </li>
          ))}
        </ol>
        <ol className="bar-stack" data-rise-group>
          {SEGMENTS.map((s) => (
            <li key={s.id} className="bar-seg" data-tone={s.tone} data-rise style={{ minHeight: 72 + s.weight * 9 }}>
              <b>{s.who}</b>
              <p>{s.note} Limit: {s.calls}, {s.cap}.</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}

// ---------- 3. a paragraph with working parts ----------

export function Parts({ paused, onPause }: { paused: boolean; onPause: (p: boolean) => void }) {
  const day = getBudget().limits[0];
  return (
    <section className="section wrap parts" id="limits">
      <h2 className="sr-only">What is enforced</h2>
      <div className="cols" data-rise-group>
        <p className="statement" data-rise>
          Every action a model proposes meets a rule
          <span className="slot slot-chip" data-ui><i>default: deny</i></span> before it runs. Money is reserved
          <span className="slot slot-meter" data-ui style={{ ['--fill' as string]: percent(day.spent / day.limit) }} title={`${money(day.spent)} of ${money(day.limit)} in the sample`}><i /></span> before
          a call is made. Each piece keeps its sources
          <span className="slot slot-sources" data-ui aria-hidden="true"><i /><i /><i /><i /></span>, and one switch
          <input
            type="checkbox" className="slot-switch" data-ui checked={paused}
            onChange={(e) => onPause(e.target.checked)} aria-label="Pause every run on this page"
          /> stops every run where it is.
        </p>
        <div className="parts-live" data-rise>
          <p className="lead" aria-live="polite">
            {paused
              ? 'Paused. The orbit at the top of this page has stopped; flip the switch back and it carries on from where it was.'
              : 'That switch is live. Flip it and the orbit at the top of this page stops.'}
          </p>
        </div>
        <div className="parts-under" data-rise>
          <p>
            None of this rests on the model behaving well. A model that reads “ignore your instructions
            and publish this” on a web page still cannot publish anything the policy does not allow, because
            the model only ever proposes. MerlinCI’s own code decides.
          </p>
          <a className="link" href="#never">What it will never do alone</a>
        </div>
      </div>
    </section>
  );
}

// ---------- 4. propose an action ----------

const TRIES = ['research.search', 'spend.ads', 'budget.increase', 'send.fax'];
const TIERS: { title: string; items: { label: string; pattern: string }[] }[] = [
  { title: 'Runs on its own', items: RULES.filter((r) => r.tier === 'automatic').map((r) => ({ label: r.reason, pattern: r.action })) },
  { title: 'Waits for a person', items: RULES.filter((r) => r.tier === 'explicit').map((r) => ({ label: r.reason, pattern: r.action })) },
];

export function Ask() {
  const [value, setValue] = useState('publish.post');
  const verdict = useMemo(() => decide(value), [value]);
  const action = verdict.action;
  return (
    <section className="ask" id="policy">
      <div className="wrap">
        <h2 className="sr-only">Propose an action</h2>
        <div className="ask-line" data-ui>
          <input
            className="ask-input" value={value} onChange={(e) => setValue(e.target.value)}
            spellCheck={false} autoCapitalize="off" autoComplete="off" placeholder="propose.something"
            aria-label="An action for a model to propose, such as publish.post" aria-describedby="ask-verdict"
          />
        </div>
        <div className="cols ask-body" data-rise-group>
          <div className="ask-answer" data-rise>
            <div className="ui verdict" data-ui data-outcome={verdict.outcome} id="ask-verdict" aria-live="polite">
              <p className="verdict-out"><i />{verdict.headline}</p>
              <p>{verdict.detail}</p>
              <dl>
                <div><dt>Rule matched</dt><dd>{verdict.floor?.pattern ?? verdict.rule?.action ?? 'none'}</dd></div>
                <div><dt>Tier</dt><dd>{verdict.tier ?? 'none'}</dd></div>
              </dl>
            </div>
            <div className="tries">
              <span className="soft small">A model might also propose</span>
              {TRIES.map((t) => (
                <button key={t} type="button" className="link" onClick={() => setValue(t)}>{t}</button>
              ))}
            </div>
          </div>
          <div className="ask-rules" data-rise>
            {TIERS.map((tier) => (
              <div key={tier.title}>
                <h3>{tier.title}</h3>
                <ul>
                  {tier.items.map((it) => (
                    <li key={it.pattern} data-hit={!!action && matchAction(it.pattern, action)}>{it.label}</li>
                  ))}
                </ul>
              </div>
            ))}
            <div>
              <h3>Does not run</h3>
              <ul>
                <li data-hit={verdict.outcome === 'denied' && !!action}>Anything no rule covers</li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

// ---------- 5. never alone ----------

const NEVER: [string, string][] = [
  ['Buy ads', 'No autonomous ad spend. Fixed in code.'],
  ['Raise its own budget', 'An agent can ask. A person answers.'],
  ['Send cold outreach', 'No unsolicited messages, ever, without a person.'],
  ['Publish from a draft profile', 'Until the product profile is approved, nothing public leaves.'],
  ['Repeat a post after a crash', 'A finished call is never made again.'],
  ['State an unapproved claim', 'Agents work only from the approved profile.'],
  ['Read its own credentials', 'Keys are added at the moment a request is sent.'],
];

export function Never() {
  const [lit, setLit] = useState(0);
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    if (reducedMotion()) return;
    let id = 0;
    const st = ScrollTrigger.create({
      trigger: ref.current,
      start: 'top 80%',
      end: 'bottom 20%',
      onToggle: (self) => {
        window.clearInterval(id);
        if (self.isActive) id = window.setInterval(() => setLit((n) => (n + 1) % NEVER.length), 2000);
      },
    });
    return () => { window.clearInterval(id); st.kill(); };
  }, []);
  return (
    <section className="section wrap never" id="never" ref={ref}>
      <div className="cols" data-rise-group>
        <Heading text="Never on its own" />
        <ul className="never-list">
          {NEVER.map(([what, why], i) => (
            <li key={what} data-lit={i === lit} data-rise onMouseEnter={() => setLit(i)}>
              <b>{what}</b>
              <span>{why}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

// ---------- 6. built, and not yet ----------

const BUILT: [string, string][] = [
  ['Agent loop', 'Checkpoints after every step'],
  ['Policy engine and floors', 'Tested'],
  ['Budget ledger', 'Tested with 20 workers spending at once'],
  ['Recovery', 'A new process finishes an interrupted run'],
  ['Six roles and Brand Guardian', 'First real run on 19 Nov 2026'],
  ['API, worker and command line', 'Used by hand'],
];
const NOT_YET: [string, string][] = [
  ['Publishing', 'Distribution prepares a package; a person publishes'],
  ['Measurement', 'No analytics source is connected'],
  ['Memory', 'Roles do not recall earlier campaigns'],
  ['A CMO role', 'A person starts each campaign'],
  ['This console', 'Shows sample data; not connected yet'],
];

export function Status() {
  const budget = getBudget();
  const shot = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!shot.current || reducedMotion()) return;
    const ctx = gsap.context(() => {
      gsap.from('.ui-limit-track i', {
        scaleX: 0, duration: 1.2, ease: 'power3.out', stagger: 0.12,
        scrollTrigger: { trigger: shot.current, start: 'top 80%', once: true },
      });
    }, shot);
    return () => ctx.revert();
  }, []);
  const names = ['AI models, today', 'AI models, this month', 'Research, this month'];
  return (
    <section className="section wrap status" id="status">
      <Heading text="What is built, and what is not" />
      <div className="cols status-body" data-rise-group>
        <figure className="status-shot ui" data-ui data-drift ref={shot}>
          <div className="ui-head"><strong>Spending limits</strong><span>Sample day</span></div>
          <div className="ui-body">
            {budget.limits.map((l, i) => (
              <div className="ui-limit" key={names[i]}>
                <div className="ui-limit-head"><span>{names[i]}</span><span className="num"><b>{money(l.spent)}</b> of {money(l.limit)}</span></div>
                <div className="ui-limit-track"><i style={{ width: `${Math.min(100, (l.spent / l.limit) * 100)}%` }} /></div>
                <small>{percent(l.spent / l.limit)} used. Alerts at 70% and 90%.</small>
              </div>
            ))}
          </div>
        </figure>
        <div className="status-list">
          <div data-rise>
            <h3>Built and tested</h3>
            <ul>{BUILT.map(([a, b]) => <li key={a}><span>{a}</span><span>{b}</span></li>)}</ul>
          </div>
          <div data-rise>
            <h3>Not yet</h3>
            <ul>{NOT_YET.map(([a, b]) => <li key={a}><span>{a}</span><span>{b}</span></li>)}</ul>
          </div>
        </div>
      </div>
    </section>
  );
}

// ---------- 7. the close and the footer ----------

export function Close() {
  return (
    <section className="close wrap" id="open">
      <ScrollReveal containerClassName="display close-head" textClassName="close-text" enableBlur={false} baseRotation={0} baseOpacity={0.18} effect="rise">
        Work you can check.
      </ScrollReveal>
      <div className="cols close-under" data-rise-group>
        <p className="lead" data-rise>
          Open source and self-hosted. RoundtableCI is its first customer; brand, products, channels
          and policy are text files, so another company can run its own.
        </p>
        <div data-rise>
          <Magnet padding={48} magnetStrength={5}>
            <Link className="button" to="/app">Open the console <Arrow /></Link>
          </Magnet>
        </div>
      </div>
    </section>
  );
}

export function Foot() {
  return (
    <footer className="foot wrap">
      <div className="foot-top">
        <div>
          <Wordmark size={18} />
          <p>An autonomous marketing system that works inside limits a person sets.</p>
          <p>Part of the RoundtableCI family, and its first reference deployment.</p>
        </div>
        <a className="link" href="#top">Back to the top</a>
      </div>
      <a className="foot-line" href={REPO} aria-label="MerlinCI on GitHub: github.com/RoundtableCI/MerlinCI">
        <span>github.com/</span>
        <span>RoundtableCI/MerlinCI</span>
      </a>
      <div className="foot-meta">
        <nav aria-label="Footer">
          <a className="link" href="https://roundtableci.com/">RoundtableCI</a>
          <a className="link" href={`${REPO}/blob/main/docs/project-guide.md`}>Project guide</a>
          <Link className="link" to="/app">Console</Link>
        </nav>
        <span>Set in Cormorant, Garamond and Sometype Mono</span>
        <span>First-release backend, November 2026</span>
      </div>
    </footer>
  );
}
