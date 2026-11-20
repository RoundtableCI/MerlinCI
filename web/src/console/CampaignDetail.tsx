// One campaign. The one question: where did this campaign stop, and why?
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ROLES, getArtifact, useCampaign, useEvents, useTasks } from '../data/api';
import type { Artifact, Campaign, CampaignStep, Evaluation, Task } from '../data/types';
import { clock, count, duration, money, roleLabel, when } from '../lib/format';
import { Icon, Panel, StatusPill, StepTrack, statusLabel } from './parts';
import { RequestCard } from './RequestCard';

const evaluatorLabel = (name: string) => (name === 'evidence' ? 'Evidence' : roleLabel(name));
const kindLabel = (kind: string) => {
  const words = kind.replace(/-/g, ' ');
  return words.charAt(0).toUpperCase() + words.slice(1);
};
const sentence = (s: string) => {
  const t = s.charAt(0).toUpperCase() + s.slice(1);
  return /[.!?]$/.test(t) ? t : `${t}.`;
};

function Crumb() {
  return (
    <Link className="crumb" to="/app/campaigns">
      Campaigns <Icon name="chevron" />
    </Link>
  );
}

/** One evaluator's verdict. `said` is text already printed above it, so it is not printed twice. */
function Verdict({ e, said }: { e: Evaluation; said?: string }) {
  return (
    <li>
      <span className={`pill ${e.verdict === 'pass' ? 'pos' : 'neg'}`}>
        {evaluatorLabel(e.evaluator)}: {e.verdict === 'pass' ? 'passed' : 'rejected'}
      </span>
      {e.feedback && e.feedback !== said && <span>{e.feedback}</span>}
    </li>
  );
}

/** Why a step has no artifact to show, in one true sentence. */
function whyNoArtifact(campaign: Campaign, step: CampaignStep, task: Task | undefined): string {
  if (step.status === 'pending') return 'This step has not started. It runs when the steps before it are completed.';
  if (step.status === 'skipped') {
    const cause = campaign.steps.find((s) => s.status === 'rejected' || s.status === 'failed');
    return cause
      ? `This step did not run because “${cause.label}” ${cause.status === 'rejected' ? 'was rejected' : 'failed'}.`
      : 'This step did not run because an earlier step stopped.';
  }
  if (step.artifactId) return `The run stored an artifact (${step.artifactId}), but its text is not in the sample data.`;
  if (!task) return 'This step has no run in the sample data, so there is no artifact.';
  if (task.run.status === 'failed') return 'The run failed, so no artifact was stored.';
  if (task.run.status === 'completed') return 'The run completed, but no artifact is recorded for it in the sample data.';
  return 'No artifact yet: the run has not finished.';
}

function StepOutput({ campaign, step, task, artifact }: {
  campaign: Campaign; step: CampaignStep; task: Task | undefined; artifact: Artifact | undefined;
}) {
  // A check (the brand check) stores nothing of its own: it gives a verdict on the artifact before it.
  if (!step.produces && step.evaluator) {
    const index = campaign.steps.indexOf(step);
    const checked = getArtifact([...campaign.steps.slice(0, index)].reverse().find((s) => s.artifactId)?.artifactId);
    const verdicts = checked?.evaluations.filter((e) => e.evaluator === step.evaluator) ?? [];
    if (step.status === 'pending' || step.status === 'skipped') {
      return <div className="empty">{whyNoArtifact(campaign, step, task)}</div>;
    }
    return (
      <>
        {checked && verdicts.length > 0 && (
          <>
            <p>
              <b>{checked.title}</b>
              <span className="caption">{kindLabel(checked.kind)} by {roleLabel(checked.role)}, checked by {roleLabel(step.evaluator)}</span>
            </p>
            <ul className="evals">{verdicts.map((e) => <Verdict key={e.evaluator} e={e} said={step.reason} />)}</ul>
          </>
        )}
        <div className="empty">
          {checked && verdicts.length > 0
            ? `${roleLabel(step.evaluator)} gives a verdict on the asset and stores no artifact of its own.`
            : `${roleLabel(step.evaluator)} stores no artifact of its own, and the text of the asset it checked is not in the sample data.`}
        </div>
      </>
    );
  }
  if (!artifact) return <div className="empty">{whyNoArtifact(campaign, step, task)}</div>;
  return (
    <>
      <p>
        <b>{artifact.title}</b>
        <span className="caption">
          {kindLabel(artifact.kind)} · stored {when(artifact.createdAt)} · cost {money(artifact.cost)}
        </span>
      </p>
      <pre className="artifact-text">{artifact.content}</pre>
      {artifact.evaluations.length > 0 && (
        <ul className="evals">{artifact.evaluations.map((e) => <Verdict key={e.evaluator} e={e} />)}</ul>
      )}
    </>
  );
}

export default function CampaignDetail() {
  const { id } = useParams();
  const campaign = useCampaign(id);
  const tasks = useTasks();
  // The choice belongs to one campaign: opening another starts from its own stopping point.
  const [picked, setPicked] = useState<{ campaign?: string; step: string }>();

  const steps = campaign?.steps ?? [];
  const fallback = steps.find((s) => s.status !== 'completed') ?? steps[steps.length - 1];
  const pickedStep = picked && picked.campaign === id ? picked.step : undefined;
  const step = steps.find((s) => s.id === pickedStep) ?? fallback;
  const task = tasks.find((t) => t.id === step?.taskId);
  const events = useEvents(task?.id);

  if (!campaign || !step) {
    return (
      <div className="title-row">
        <div>
          <Crumb />
          <h1>Campaign not found</h1>
          <p className="sub">No campaign has the id <span className="mono">{id}</span>.</p>
        </div>
      </div>
    );
  }

  const artifact = getArtifact(step.artifactId);
  const role = ROLES.find((r) => r.name === task?.role);
  const waiting = task?.run.status === 'awaiting_approval' && task.run.pending;

  return (
    <>
      <div className="title-row">
        <div>
          <Crumb />
          <h1>{campaign.topic}</h1>
          <p className="sub">
            <b>{statusLabel(campaign.status)}.</b>{' '}
            {campaign.reason ? sentence(campaign.reason) : <span className="muted">Started {when(campaign.createdAt)}, last change {when(campaign.updatedAt)}.</span>}
          </p>
        </div>
      </div>

      <div className="grid top">
        <Panel span={12} title="Steps" meta={`${campaign.workflow} · started ${when(campaign.createdAt)}`}>
          <StepTrack
            steps={campaign.steps} selected={step.id} onSelect={(s) => setPicked({ campaign: id, step: s })}
            spentOf={(s) => tasks.find((t) => t.id === s.taskId)?.run.spent}
          />
        </Panel>

        <Panel span={7} title={step.label} meta={statusLabel(step.status)}>
          <div className="stack">
            {waiting && task && <RequestCard task={task} />}
            {step.reason && <p>{step.reason}</p>}
            <StepOutput campaign={campaign} step={step} task={task} artifact={artifact} />
          </div>
        </Panel>

        <div className="stack" style={{ gridColumn: 'span 5' }}>
          <Panel title="Run" meta={task ? <span className="mono">{task.id}</span> : undefined}>
            {task ? (
              <div className="stack">
                <dl className="kv rows">
                  <div><dt>Role</dt><dd>{roleLabel(task.role)}</dd></div>
                  <div><dt>Model</dt><dd className="mono">{role?.model ?? <span className="muted">–</span>}</dd></div>
                  <div><dt>Status</dt><dd><StatusPill status={task.run.status} /></dd></div>
                  <div><dt>Model calls</dt><dd>{task.run.step} <span className="of">/{task.run.maxSteps}</span></dd></div>
                  <div><dt>Spent</dt><dd>{money(task.run.spent)} <span className="of">of {money(task.run.cap)}</span></dd></div>
                  <div><dt>Tokens in / out</dt><dd>{count(task.run.inputTokens)} <span className="of">/ {count(task.run.outputTokens)}</span></dd></div>
                  <div><dt>Started</dt><dd>{when(task.createdAt)}</dd></div>
                  <div>
                    <dt>{task.endedAt ? 'Took' : 'Running for'}</dt>
                    <dd>{duration(task.createdAt, task.endedAt)}</dd>
                  </div>
                  <div><dt>Configuration</dt><dd className="mono">{task.run.configDigest}</dd></div>
                </dl>
                {task.run.reason && <p className="muted">{sentence(task.run.reason)}</p>}
              </div>
            ) : (
              <div className="empty">
                {step.status === 'pending' || step.status === 'skipped'
                  ? 'No run: this step did not start.'
                  : 'No run is recorded for this step in the sample data.'}
              </div>
            )}
          </Panel>

          <Panel title="Sources" meta={artifact ? `${count(artifact.sources.length)} fetched` : undefined}>
            {artifact && artifact.sources.length > 0 ? (
              <ul className="sources">
                {artifact.sources.map((s) => (
                  <li key={s.url}>
                    <a href={s.url} target="_blank" rel="noreferrer">{s.title}</a>
                    <span className="muted">{s.url}</span>
                    <span>{s.excerpt}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="empty">
                {!task
                  ? 'No run, so no sources.'
                  : task.run.sources > 0
                    ? `The run carries ${count(task.run.sources)} sources. Their pages are not in the sample data.`
                    : 'This run saved no sources.'}
              </div>
            )}
          </Panel>
        </div>

        <Panel span={12} title="Run record" meta={events ? `${count(events.length)} events, oldest first` : undefined}>
          {events && events.length > 0 ? (
            <ol className="record">
              {events.map((e, i) => (
                <li key={`${e.at}-${i}`} data-tone={e.tone}>
                  <time dateTime={e.at}>{clock(e.at)}</time>
                  <strong>{e.type}</strong>
                  <p>{e.detail}</p>
                </li>
              ))}
            </ol>
          ) : (
            <div className="empty">
              {task ? 'The record for this run is not in the sample data.' : 'This step has no run, so it has no record.'}
            </div>
          )}
        </Panel>
      </div>
    </>
  );
}
