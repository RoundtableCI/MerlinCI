// Overview. The one question: is anything waiting on me?
import { Link } from 'react-router-dom';
import { useBudget, useCampaigns, usePending, useTasks } from '../data/api';
import { count, money, roleLabel, when } from '../lib/format';
import { LimitTrack, Panel, StatusPill, StepTrack } from './parts';
import { RequestCard, describeRequest } from './RequestCard';

export default function Overview() {
  const pending = usePending();
  const tasks = useTasks();
  const campaigns = useCampaigns();
  const budget = useBudget();

  const current = campaigns.find((c) => c.status === 'awaiting_approval' || c.status === 'running') ?? campaigns[0];
  const spentOf = (taskId?: string) => tasks.find((t) => t.id === taskId)?.run.spent;
  const recent = [...tasks].sort((a, b) => b.createdAt.localeCompare(a.createdAt)).slice(0, 5);
  const first = pending[0];

  return (
    <>
      <div className="title-row">
        <div>
          <h1>Overview</h1>
          <p className="sub">
            {first ? (
              <>
                <b>{pending.length === 1 ? 'One action is waiting on you.' : `${pending.length} actions are waiting on you.`}</b>{' '}
                {roleLabel(first.role)} asks: {describeRequest(first).toLowerCase()}.
              </>
            ) : (
              <>Nothing is waiting on you. <span className="muted">Runs continue inside the policy and the budget.</span></>
            )}
          </p>
        </div>
        <div className="title-actions">
          <Link className="btn" to="/app/campaigns">All campaigns</Link>
          {first && <Link className="btn primary" to="/app/approvals">Review the request</Link>}
        </div>
      </div>

      <div className="grid">
        <Panel
          span={7} title="Waiting on you"
          meta={first ? `${pending.length} of ${pending.length}` : undefined}
          action={<Link className="card-link" to="/app/approvals">All approvals</Link>}
        >
          {first ? (
            <RequestCard task={first} />
          ) : (
            <div className="empty">No action needs a person right now. Decisions you made stay in the record under Approvals.</div>
          )}
        </Panel>

        <Panel span={5} title="Spending limits" action={<Link className="card-link" to="/app/budget">Budget</Link>}>
          <div className="limits">
            {budget.limits.map((l) => <LimitTrack key={`${l.category}/${l.period}`} line={l} thresholds={budget.alertThresholds} />)}
          </div>
        </Panel>

        <Panel
          span={12} title={current.topic} meta={`first-release · started ${when(current.createdAt)}`}
          action={<Link className="card-link" to={`/app/campaigns/${current.id}`}>Open the campaign</Link>}
        >
          <StepTrack steps={current.steps} spentOf={(s) => spentOf(s.taskId)} />
        </Panel>

        <Panel span={12} title="Recent runs" meta={`${count(tasks.length)} this week`} action={<Link className="card-link" to="/app/campaigns">All campaigns</Link>}>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Role and topic</th>
                  <th>Status</th>
                  <th className="n">Model calls</th>
                  <th className="n">Sources</th>
                  <th className="n">Spent</th>
                  <th className="n">Started</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((t) => (
                  <tr key={t.id}>
                    <td>
                      {t.campaignId
                        ? <Link className="name" to={`/app/campaigns/${t.campaignId}`}>{roleLabel(t.role)}</Link>
                        : <span className="name" style={{ fontWeight: 500 }}>{roleLabel(t.role)}</span>}
                      <span className="sub">{String(t.input.topic)}</span>
                    </td>
                    <td><StatusPill status={t.run.status} /></td>
                    <td className="n">{t.run.step} <span className="of">/{t.run.maxSteps}</span></td>
                    <td className="n">{t.run.sources}</td>
                    <td className="n">{money(t.run.spent)} <span className="of">/{money(t.run.cap)}</span></td>
                    <td className="n muted">{when(t.createdAt)}</td>
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
