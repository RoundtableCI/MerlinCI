// Rules. The one question: what may run without me?
import { getConfig } from '../data/api';
import { count, money, roleLabel } from '../lib/format';
import { FLOORS, PUBLIC_PREFIXES, RULES, decide, matchAction } from '../lib/policy';
import type { Tier } from '../lib/policy';
import { Panel } from './parts';

const TIER: Record<Tier, { label: string; tone: string }> = {
  automatic: { label: 'Automatic', tone: '' },
  conditional: { label: 'Conditional', tone: '' },
  explicit: { label: 'Explicit', tone: 'warn' },
};
const ORDER: Tier[] = ['automatic', 'conditional', 'explicit'];

const evaluatorLabel = (name: string) => (name === 'evidence' ? 'Evidence' : roleLabel(name));
const dash = <span className="muted">–</span>;
const list = (items: string[]) =>
  items.length < 2 ? items.join('') : `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;

export default function Rules() {
  const config = getConfig();
  const draft = !config.profileApproved;

  const rules = ORDER.flatMap((tier) => RULES.filter((r) => r.tier === tier)).map((rule) => {
    const base = rule.action.replace(/\.\*$/, '');
    return {
      rule,
      floor: FLOORS.find((f) => matchAction(f.pattern, base)),
      verdict: decide(base, { profileApproved: config.profileApproved }),
    };
  });
  const runs = rules.filter((r) => r.verdict.outcome === 'runs').length;
  const waits = rules.filter((r) => r.verdict.outcome === 'waits').length;
  const refused = rules.filter((r) => r.verdict.outcome === 'refused').length;
  const conditional = RULES.filter((r) => r.tier === 'conditional').length;

  return (
    <>
      <div className="title-row">
        <div>
          <h1>Rules</h1>
          <p className="sub">
            <b>
              {draft
                ? 'The product profile is a draft, so every public-facing action is refused.'
                : 'The product profile is approved, so public-facing actions wait for a person instead of being refused.'}
            </b>{' '}
            Of {count(rules.length)} rules, {count(runs)} run without you, {count(waits)} wait for a person{refused ? ` and ${count(refused)} are refused today` : ''}.
          </p>
        </div>
      </div>

      <div className="grid top">
        <Panel span={7} title="Policy" meta={`${count(rules.length)} rules`}>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Action</th>
                  <th>Tier</th>
                  <th>What it covers</th>
                  <th>Today</th>
                </tr>
              </thead>
              <tbody>
                {rules.map(({ rule, floor, verdict }) => (
                  <tr key={rule.action}>
                    <td>
                      <span className="mono">{rule.action}</span>
                      {floor && <span className="sub">Fixed in code</span>}
                    </td>
                    <td><span className={`pill ${TIER[rule.tier].tone}`}>{TIER[rule.tier].label}</span></td>
                    <td>
                      {rule.reason}
                      {floor && <span className="sub">{floor.reason}</span>}
                    </td>
                    <td>{verdict.headline}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="muted note">
            {conditional === 0 && 'No rule is in the conditional tier today. '}
            Anything else: denied. An action no rule covers does not run.
          </p>
        </Panel>

        <div className="stack" style={{ gridColumn: 'span 5' }}>
          <Panel title="Product profile">
            <div className="stack">
              <dl className="kv rows">
                <div><dt>Name</dt><dd className="mono">{config.profile}</dd></div>
                <div>
                  <dt>Status</dt>
                  <dd><span className={`pill ${draft ? 'warn' : 'pos'}`}>{draft ? 'Draft' : 'Approved'}</span></dd>
                </div>
              </dl>
              <p>
                {draft
                  ? `While the profile is a draft, every action that puts something in front of people outside the company is refused: ${list(PUBLIC_PREFIXES)}.`
                  : `The profile is approved, so actions that put something in front of people outside the company (${list(PUBLIC_PREFIXES)}) follow the policy.`}
              </p>
            </div>
          </Panel>

          <Panel title="Configuration">
            <div className="stack">
              <dl className="kv rows">
                <div><dt>Digest</dt><dd className="mono">{config.digest}</dd></div>
              </dl>
              <p>Every run records the digest of the configuration it ran with.</p>
            </div>
          </Panel>
        </div>

        <Panel span={12} title="Roles" meta={`${count(config.roles.filter((r) => r.enabled).length)} of ${count(config.roles.length)} enabled`}>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Role</th>
                  <th>Model</th>
                  <th>Tools</th>
                  <th>Checked by</th>
                  <th className="n">Model calls</th>
                  <th className="n">Cap per task</th>
                  <th>On or off</th>
                </tr>
              </thead>
              <tbody>
                {config.roles.map((r) => (
                  <tr key={r.name}>
                    <td>
                      <span style={{ fontWeight: 500 }}>{r.label}</span>
                      <span className="sub">{r.responsibility}</span>
                    </td>
                    <td className="mono">{r.model}</td>
                    <td>{r.tools.length ? <span className="mono">{r.tools.join(', ')}</span> : dash}</td>
                    <td>{r.evaluators.length ? r.evaluators.map(evaluatorLabel).join(', ') : dash}</td>
                    <td className="n">{count(r.maxSteps)}</td>
                    <td className="n">{money(r.maxCostPerTask)}</td>
                    <td><span className={`pill ${r.enabled ? 'pos' : ''}`}>{r.enabled ? 'Enabled' : 'Disabled'}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>
    </>
  );
}
