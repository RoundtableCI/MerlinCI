// Campaigns. The one question: which campaigns need attention?
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useCampaigns, useTasks } from '../data/api';
import type { Campaign } from '../data/types';
import { count, money, shortId, when } from '../lib/format';
import { Panel, StatusPill } from './parts';

type Filter = 'all' | 'waiting' | 'completed' | 'stopped';

const FILTERS: { id: Filter; label: string; test: (c: Campaign) => boolean; none: string }[] = [
  { id: 'all', label: 'All', test: () => true, none: 'No campaign has been started.' },
  { id: 'waiting', label: 'Waiting on you', test: (c) => c.status === 'awaiting_approval', none: 'No campaign is waiting on you.' },
  { id: 'completed', label: 'Completed', test: (c) => c.status === 'completed', none: 'No campaign has completed.' },
  {
    id: 'stopped', label: 'Stopped or failed',
    test: (c) => c.status === 'stopped' || c.status === 'failed' || c.status === 'cancelled',
    none: 'No campaign has stopped or failed.',
  },
];

const join = (parts: string[]) =>
  parts.length < 2 ? parts.join('') : `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;

export default function Campaigns() {
  const campaigns = useCampaigns();
  const tasks = useTasks();
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');

  const spentOf = (c: Campaign) => tasks.filter((t) => t.campaignId === c.id).reduce((a, t) => a + t.run.spent, 0);
  const active = FILTERS.find((f) => f.id === filter)!;
  const q = query.trim().toLowerCase();
  const rows = campaigns.filter((c) => active.test(c) && (!q || c.topic.toLowerCase().includes(q)));

  const n = (status: Campaign['status']) => campaigns.filter((c) => c.status === status).length;
  const attention = [
    n('awaiting_approval') ? `${count(n('awaiting_approval'))} ${n('awaiting_approval') === 1 ? 'is' : 'are'} waiting on you` : '',
    n('stopped') ? `${count(n('stopped'))} stopped at a gate` : '',
    n('failed') ? `${count(n('failed'))} failed` : '',
  ].filter(Boolean);

  return (
    <>
      <div className="title-row">
        <div>
          <h1>Campaigns</h1>
          <p className="sub">
            {attention.length ? (
              <>
                <b>Of {count(campaigns.length)} campaigns, {join(attention)}.</b>{' '}
                <span className="muted">{count(n('completed'))} completed all six steps.</span>
              </>
            ) : (
              <>No campaign needs attention. <span className="muted">{count(n('completed'))} of {count(campaigns.length)} completed all six steps.</span></>
            )}
          </p>
        </div>
      </div>

      <div className="tabs" role="tablist" aria-label="Campaigns by status">
        {FILTERS.map((f) => (
          <button
            key={f.id} type="button" role="tab" id={`tab-${f.id}`}
            aria-selected={filter === f.id} aria-controls="campaign-list" onClick={() => setFilter(f.id)}
          >
            {f.label} <span className="num of">{count(campaigns.filter(f.test).length)}</span>
          </button>
        ))}
      </div>

      <div className="grid">
        <Panel span={12}>
          <div id="campaign-list" role="tabpanel" aria-labelledby={`tab-${filter}`}>
            <div className="toolbar">
              <input
                className="input" type="search" value={query} onChange={(e) => setQuery(e.target.value)}
                placeholder="Search by topic" aria-label="Search campaigns by topic"
              />
              <span className="muted num">{count(rows.length)} {rows.length === 1 ? 'campaign' : 'campaigns'}</span>
            </div>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Topic</th>
                    <th>Status</th>
                    <th>Stopped at</th>
                    <th className="n">Steps done</th>
                    <th className="n">Spent</th>
                    <th className="n">Updated</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((c) => {
                    const open = c.steps.find((s) => s.status !== 'completed');
                    const done = c.steps.filter((s) => s.status === 'completed').length;
                    return (
                      <tr key={c.id}>
                        <td>
                          <Link className="name" to={`/app/campaigns/${c.id}`}>{c.topic}</Link>
                          <span className="sub mono">{shortId(c.id)}</span>
                        </td>
                        <td><StatusPill status={c.status} /></td>
                        <td>{open ? open.label : <span className="muted">–</span>}</td>
                        <td className="n">{done} <span className="of">/{c.steps.length}</span></td>
                        <td className="n">{money(spentOf(c))}</td>
                        <td className="n muted">{when(c.updatedAt)}</td>
                      </tr>
                    );
                  })}
                  {rows.length === 0 && (
                    <tr>
                      <td colSpan={6}>
                        <div className="empty">{q ? `No campaign in this list has “${query.trim()}” in its topic.` : active.none}</div>
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </Panel>
      </div>
    </>
  );
}
