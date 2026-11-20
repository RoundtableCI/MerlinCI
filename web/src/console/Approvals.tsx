// Approvals. The one question: what is waiting on me, and what did I decide before?
import { Link } from 'react-router-dom';
import { useCampaigns, useDecisions, usePending } from '../data/api';
import { count, roleLabel, when } from '../lib/format';
import { Panel } from './parts';
import { RequestCard } from './RequestCard';

export default function Approvals() {
  const pending = usePending();
  const decisions = useDecisions();
  const campaigns = useCampaigns();

  const approved = decisions.filter((d) => d.approve).length;
  const rejected = decisions.length - approved;

  return (
    <>
      <div className="title-row">
        <div>
          <h1>Approvals</h1>
          <p className="sub">
            <b>
              {pending.length === 0
                ? 'Nothing is waiting on you.'
                : pending.length === 1 ? 'One action is waiting on you.' : `${count(pending.length)} actions are waiting on you.`}
            </b>{' '}
            <span className="muted">
              {decisions.length === 0
                ? 'No decision is on record yet.'
                : `${count(decisions.length)} ${decisions.length === 1 ? 'decision is' : 'decisions are'} on record: ${count(approved)} approved, ${count(rejected)} rejected.`}
            </span>
          </p>
        </div>
      </div>

      <div className="grid top">
        <Panel span={8} title="Waiting on you" meta={pending.length ? `${count(pending.length)}, oldest first` : undefined}>
          {pending.length ? (
            <div className="stack">
              {pending.map((t) => {
                const campaign = campaigns.find((c) => c.id === t.campaignId);
                return (
                  <div key={t.id} className="pending-item">
                    <p className="muted">
                      {campaign
                        ? <>Campaign: <Link className="card-link" to={`/app/campaigns/${campaign.id}`}>{campaign.topic}</Link></>
                        : <>A run started by hand: {String(t.input.topic)}</>}
                    </p>
                    <RequestCard task={t} />
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="empty">No action needs a person right now. Runs continue inside the policy and the budget.</div>
          )}
        </Panel>

        <Panel span={4} title="How an approval works">
          <ol className="plain-list">
            <li>One approval covers one action, and it is used up when the action starts.</li>
            <li>An approval names the exact pending action you were shown. It is refused if the task has moved on.</li>
            <li>When you reject, the model is told why and continues.</li>
          </ol>
        </Panel>

        <Panel span={12} title="Decided" meta={decisions.length ? `${count(decisions.length)}, newest first` : undefined}>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Action</th>
                  <th>Asked by</th>
                  <th>Decision</th>
                  <th>Reason</th>
                  <th>By</th>
                  <th className="n">When</th>
                </tr>
              </thead>
              <tbody>
                {decisions.map((d) => (
                  <tr key={`${d.taskId}-${d.seq}`}>
                    <td className="mono">{d.action}</td>
                    <td>{roleLabel(d.role)}</td>
                    <td><span className={`pill ${d.approve ? 'pos' : 'neg'}`}>{d.approve ? 'Approved' : 'Rejected'}</span></td>
                    <td>{d.reason || <span className="muted">–</span>}</td>
                    <td>{d.by}</td>
                    <td className="n muted">{when(d.at)}</td>
                  </tr>
                ))}
                {decisions.length === 0 && (
                  <tr>
                    <td colSpan={6}><div className="empty">No decision is on record yet.</div></td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>
    </>
  );
}
