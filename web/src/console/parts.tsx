// Small pieces every console screen shares.
import type { ReactNode } from 'react';
import type { BudgetLine, CampaignStatus, CampaignStep, RunStatus, StepStatus } from '../data/types';
import { ROLES, WORKFLOW } from '../data/api';
import { money, percent, roleLabel } from '../lib/format';

// ---------- status ----------

const STATUS: Record<RunStatus | CampaignStatus | StepStatus, { label: string; tone: '' | 'pos' | 'neg' | 'warn' }> = {
  running: { label: 'Running', tone: '' },
  awaiting_approval: { label: 'Waiting on you', tone: 'warn' },
  completed: { label: 'Completed', tone: 'pos' },
  failed: { label: 'Failed', tone: 'neg' },
  cancelled: { label: 'Cancelled', tone: '' },
  pending: { label: 'Not started', tone: '' },
  stopped: { label: 'Stopped at a gate', tone: 'neg' },
  rejected: { label: 'Rejected', tone: 'neg' },
  skipped: { label: 'Did not run', tone: '' },
};

export const statusLabel = (s: keyof typeof STATUS) => STATUS[s].label;

export function StatusPill({ status }: { status: keyof typeof STATUS }) {
  const s = STATUS[status];
  return <span className={`pill ${s.tone}`}>{s.label}</span>;
}

// ---------- the step track: one campaign as one bar ----------
// MerlinCI's own element. Each step's width follows the most model calls its role may make
// (configs/roles), so the bar shows where a campaign's effort can go. The brand check is one call.

export const stepWeight = (step: { role?: string }) => ROLES.find((r) => r.name === step.role)?.maxSteps ?? 2;

export function StepTrack({
  steps, selected, onSelect, spentOf,
}: {
  steps: CampaignStep[];
  selected?: string;
  onSelect?: (id: string) => void;
  spentOf?: (step: CampaignStep) => number | undefined;
}) {
  return (
    <ol className="steptrack" aria-label="Campaign steps">
      {steps.map((s) => {
        const spent = spentOf?.(s);
        const body = (
          <>
            <span className="steptrack-label">{s.label}</span>
            <span className="steptrack-who">{roleLabel(s.role ?? s.evaluator)}</span>
            <span className="steptrack-foot">
              <span>{statusLabel(s.status)}</span>
              {spent != null && <span className="num">{money(spent)}</span>}
            </span>
          </>
        );
        return (
          <li key={s.id} className="steptrack-step" data-status={s.status} style={{ flexGrow: stepWeight(s) }}>
            {onSelect ? (
              <button type="button" aria-pressed={selected === s.id} onClick={() => onSelect(s.id)}>{body}</button>
            ) : (
              <div>{body}</div>
            )}
          </li>
        );
      })}
    </ol>
  );
}

export const workflowStep = (id: string) => WORKFLOW.find((w) => w.id === id);

// ---------- a spending limit as a track ----------

const LIMIT_NAME: Record<string, string> = {
  'inference/day': 'AI models, today',
  'inference/month': 'AI models, this month',
  'research/month': 'Research, this month',
};
export const limitName = (l: BudgetLine) => LIMIT_NAME[`${l.category}/${l.period}`];

export function LimitTrack({ line, thresholds = [0.7, 0.9] }: { line: BudgetLine; thresholds?: number[] }) {
  const ratio = line.limit ? line.spent / line.limit : 0;
  const crossed = thresholds.filter((t) => ratio >= t).length;
  return (
    <div className="limit">
      <div className="limit-head">
        <span>{limitName(line)}</span>
        <span className="num">
          <b>{money(line.spent)}</b> <span className="muted">of {money(line.limit)}</span>
        </span>
      </div>
      <div className="limit-track" role="img" aria-label={`${percent(ratio)} of the limit used`}>
        <span className="limit-fill" data-level={crossed} style={{ width: `${Math.min(100, ratio * 100)}%` }} />
        {thresholds.map((t) => (
          <span key={t} className="limit-tick" style={{ left: `${t * 100}%` }} />
        ))}
      </div>
      <div className="limit-foot muted">
        <span>{percent(ratio)} used</span>
        <span>{crossed === 0 ? `Alerts at ${thresholds.map(percent).join(' and ')}` : `${percent(thresholds[crossed - 1])} alert raised`}</span>
      </div>
    </div>
  );
}

// ---------- layout helpers ----------

export function Panel({ title, meta, action, children, span, className = '' }: {
  title?: string; meta?: ReactNode; action?: ReactNode; children: ReactNode; span?: number; className?: string;
}) {
  return (
    <section className={`panel ${className}`} style={span ? { gridColumn: `span ${span}` } : undefined}>
      {(title || action) && (
        <header>
          <div className="panel-title">
            {title && <h2>{title}</h2>}
            {meta && <span className="meta">{meta}</span>}
          </div>
          {action}
        </header>
      )}
      {children}
    </section>
  );
}

/** Label and value pairs in a quiet well. */
export function KeyValues({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <dl className="kv">
      {rows.map(([k, v]) => (
        <div key={k}>
          <dt>{k}</dt>
          <dd>{v}</dd>
        </div>
      ))}
    </dl>
  );
}

// ---------- icons: one outline set, 16px ----------

const paths: Record<string, ReactNode> = {
  overview: <><rect x="2.5" y="2.5" width="4.5" height="5" rx="1" /><rect x="9" y="2.5" width="4.5" height="3" rx="1" /><rect x="2.5" y="9.5" width="4.5" height="4" rx="1" /><rect x="9" y="7.5" width="4.5" height="6" rx="1" /></>,
  campaigns: <><path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h7" /></>,
  approvals: <><circle cx="8" cy="8" r="5.5" /><path d="M5.6 8.2 7.3 9.9l3.2-3.6" /></>,
  budget: <><path d="M2.5 13.5V9M6.2 13.5V5.5M9.8 13.5V8M13.5 13.5V2.5" /></>,
  rules: <><path d="M8 2 3 4v3.6c0 3 2 5.3 5 6.4 3-1.1 5-3.4 5-6.4V4L8 2Z" /></>,
  out: <><path d="M6 3.5H3.5v9H12V10M9 3.5h3.5V7M12.5 3.5 7.5 8.5" /></>,
  menu: <><path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11" /></>,
  close: <><path d="m4 4 8 8M12 4l-8 8" /></>,
  chevron: <><path d="m6 3.5 4.5 4.5L6 12.5" /></>,
};

export function Icon({ name }: { name: keyof typeof paths | string }) {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths[name]}
    </svg>
  );
}
