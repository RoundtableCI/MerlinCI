// Budget. The one question: how much of each limit is used?
import { useEffect, useRef, useState } from 'react';
import { NOW, inferenceByDay, searchesThisMonth, spendByRole, useBudget, useTasks } from '../data/api';
import type { DaySpend } from '../data/api';
import { count, dayMonth, money, percent } from '../lib/format';
import { KeyValues, LimitTrack, Panel } from './parts';

const TODAY = NOW.slice(0, 10);

/** Spend per day as columns, drawn at one unit per pixel so the 12px labels stay 12px at any width. */
function DayChart({ days, limit, alertAt }: { days: DaySpend[]; limit: number; alertAt: number }) {
  const box = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(720);
  useEffect(() => {
    const el = box.current;
    if (!el || typeof ResizeObserver === 'undefined') return;
    const ro = new ResizeObserver(([entry]) => setWidth(Math.max(260, Math.round(entry.contentRect.width))));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const height = 300;
  const left = 60, top = 44, bottom = 26;
  const plotW = width - left;
  const plotH = height - top - bottom;
  const max = Math.max(limit, ...days.map((d) => d.spent));
  const y = (v: number) => top + plotH * (1 - v / max);
  const band = plotW / days.length;
  const barW = Math.min(28, band * 0.68);
  const every = width < 480 ? 6 : 3;
  const ticks = [0, limit / 3, (2 * limit) / 3, limit];

  const peak = days.reduce((a, d) => (d.spent > a.spent ? d : a), days[0]);
  const today = days.find((d) => d.date === TODAY);
  const label =
    `AI model spend for each day from ${dayMonth(days[0].date)} to ${dayMonth(days[days.length - 1].date)}. ` +
    `The highest day is ${dayMonth(peak.date)} at ${money(peak.spent)}. ` +
    `${today ? `Today is ${money(today.spent)}. ` : ''}The daily limit is ${money(limit)}.`;

  return (
    <div className="barchart" ref={box}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={label}>
        {ticks.map((t) => (
          <g key={t}>
            {t !== limit && <line className="gridline" x1={left} x2={width} y1={y(t)} y2={y(t)} />}
            <text x={left - 10} y={y(t) + 4} textAnchor="end">{money(t)}</text>
          </g>
        ))}
        {days.map((d, i) => {
          const cx = left + band * i + band / 2;
          return (
            <g key={d.date}>
              {d.spent > 0 && (
                <rect
                  className={d.date === TODAY ? 'bar today' : 'bar'}
                  x={cx - barW / 2} y={y(d.spent)} width={barW} height={y(0) - y(d.spent)}
                >
                  <title>{`${dayMonth(d.date)}: ${money(d.spent)}`}</title>
                </rect>
              )}
              {d.spent >= limit * alertAt && <text x={cx} y={y(limit) - 6} textAnchor="middle">{percent(alertAt)}</text>}
              {i % every === 0 && <text x={cx} y={height - 6} textAnchor="middle">{dayMonth(d.date)}</text>}
            </g>
          );
        })}
        <line className="limitline" x1={left} x2={width} y1={y(limit)} y2={y(limit)} />
        <text x={width} y={y(limit) - 24} textAnchor="end">Daily limit {money(limit)}</text>
      </svg>
      <p className="legend">
        <span><i className="today" />Today</span>
        <span><i />Earlier days</span>
        <span>{percent(alertAt)}: the day reached the {percent(alertAt)} alert</span>
      </p>
    </div>
  );
}

export default function Budget() {
  const tasks = useTasks();
  const budget = useBudget();
  const days = inferenceByDay(tasks);
  const roles = spendByRole(tasks);
  const searches = searchesThisMonth(tasks);

  const daily = budget.limits.find((l) => l.category === 'inference' && l.period === 'day')!;
  const research = budget.limits.find((l) => l.category === 'research')!;
  const [warnAt, alertAt] = [Math.min(...budget.alertThresholds), Math.max(...budget.alertThresholds)];
  const hot = days.filter((d) => d.spent >= daily.limit * alertAt).length;
  const runs = roles.reduce((a, r) => a + r.tasks, 0);
  const total = roles.reduce((a, r) => a + r.spent, 0);

  return (
    <>
      <div className="title-row">
        <div>
          <h1>Budget</h1>
          <p className="sub">
            <b>
              AI models have spent {money(daily.spent)} of today’s {money(daily.limit)}, {percent(daily.spent / daily.limit)} of the daily limit.
            </b>{' '}
            {hot === 0
              ? `No day this month has reached ${percent(alertAt)} of it.`
              : `${count(hot)} ${hot === 1 ? 'day' : 'days'} this month reached ${percent(alertAt)} of it.`}
          </p>
        </div>
      </div>

      <div className="grid">
        <Panel span={12} title="Limits">
          <div className="stack">
            <div className="limits across">
              {budget.limits.map((l) => <LimitTrack key={`${l.category}/${l.period}`} line={l} thresholds={budget.alertThresholds} />)}
            </div>
            <KeyValues
              rows={[
                ['Searches this month', count(searches)],
                ['Cost of one search', searches ? money(research.spent / searches) : <span className="muted">–</span>],
                ['Research spend', `${money(research.spent)} of ${money(research.limit)}`],
              ]}
            />
          </div>
        </Panel>

        <Panel span={8} title="AI model spend per day, November" meta={`${count(days.length)} days`}>
          <DayChart days={days} limit={daily.limit} alertAt={alertAt} />
        </Panel>

        <Panel span={4} title="Spend by role">
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Role</th>
                  <th className="n">Runs</th>
                  <th className="n">Spent</th>
                </tr>
              </thead>
              <tbody>
                {roles.map((r) => (
                  <tr key={r.role}>
                    <td>{r.label}</td>
                    <td className="n">{count(r.tasks)}</td>
                    <td className="n">{money(r.spent)}</td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr>
                  <td><b>Total</b></td>
                  <td className="n"><b>{count(runs)}</b></td>
                  <td className="n"><b>{money(total)}</b></td>
                </tr>
              </tfoot>
            </table>
          </div>
          <p className="muted note">Only the {count(runs)} runs in the sample are counted, so this total is lower than the month’s spend.</p>
        </Panel>

        <Panel span={12} title="What the limits do">
          <dl className="terms">
            <div>
              <dt>Reserved first</dt>
              <dd>Spend is reserved before every model call. A call that does not fit under the limit is never made.</dd>
            </div>
            <div>
              <dt>Doubt counts as spent</dt>
              <dd>A call that fails in a way that might still have been billed counts as spent.</dd>
            </div>
            <div>
              <dt>Alerts</dt>
              <dd>An alert is raised once per period at {percent(warnAt)} and again at {percent(alertAt)} of a limit.</dd>
            </div>
            <div>
              <dt>Raising a limit</dt>
              <dd>Agents can never raise their own limit. A request to raise one always waits for a person.</dd>
            </div>
          </dl>
        </Panel>
      </div>
    </>
  );
}
