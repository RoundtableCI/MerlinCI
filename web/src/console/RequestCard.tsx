import { useState } from 'react';
import { decide } from '../data/api';
import type { Task } from '../data/types';
import { clock, duration, money, roleLabel } from '../lib/format';
import { KeyValues } from './parts';

/** In plain words, what a pending action asks for. */
export function describeRequest(task: Task): string {
  const p = task.run.pending!;
  if (p.action === 'budget.increase') {
    return `Raise the cap on ${String(p.input.scope)} from ${money(Number(p.input.from))} to ${money(Number(p.input.to))}`;
  }
  return `${p.tool} (${p.action})`;
}

/** One action waiting for a person, with the two answers the API accepts. */
export function RequestCard({ task }: { task: Task }) {
  const p = task.run.pending!;
  const [reason, setReason] = useState('');
  return (
    <div className="request">
      <p className="request-what">{describeRequest(task)}</p>
      <p className="request-why">{p.rationale}</p>
      <KeyValues
        rows={[
          ['Asked by', roleLabel(task.role)],
          ['Action', <span className="mono">{p.action}</span>],
          ['Spent so far', `${money(task.run.spent)} of ${money(task.run.cap)}`],
          ['Waiting', `${duration(p.askedAt)}, since ${clock(p.askedAt)}`],
        ]}
      />
      <div className="request-actions">
        <input
          className="input" value={reason} onChange={(e) => setReason(e.target.value)}
          placeholder="Reason, kept in the record (optional)" aria-label="Reason for your decision"
        />
        <button type="button" className="btn" onClick={() => decide(task.id, p.seq, false, reason)}>Reject</button>
        <button type="button" className="btn primary" onClick={() => decide(task.id, p.seq, true, reason)}>Approve once</button>
      </div>
    </div>
  );
}
