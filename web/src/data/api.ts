// The console's only door to data. Today every function answers from src/data/sample.ts and
// keeps changes (a decision, the pause switch) in memory. To connect the real backend, replace
// the bodies with fetch calls to the endpoints named beside each one; the screens do not change.

import { useSyncExternalStore } from 'react';
import {
  ARTIFACTS, CAMPAIGNS, CONFIG, DECISIONS, EVENTS, INFERENCE_BY_DAY, NOW, ROLES, SEARCH_COST, SEARCHES_BEFORE, TASKS, WORKFLOW,
} from './sample';
import type { Artifact, Budget, Campaign, Config, Decision, Task, TaskEvent } from './types';

export { NOW, ROLES, WORKFLOW };
export const IS_SAMPLE = true;

interface State {
  paused: boolean;
  tasks: Task[];
  campaigns: Campaign[];
  decisions: Decision[];
  events: Record<string, TaskEvent[]>;
}

let state: State = {
  paused: false,
  tasks: TASKS,
  campaigns: CAMPAIGNS,
  decisions: DECISIONS,
  events: EVENTS,
};

const listeners = new Set<() => void>();
const set = (next: Partial<State>) => {
  state = { ...state, ...next };
  listeners.forEach((l) => l());
};
const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};

/** Subscribe a component to the store. */
export function useStore<T>(pick: (s: State) => T): T {
  return useSyncExternalStore(subscribe, () => pick(state));
}

// ---------- reads ----------

/** GET /v1/tasks */
export const useTasks = () => useStore((s) => s.tasks);
/** GET /v1/tasks/{id} */
export const useTask = (id: string | undefined) => useStore((s) => s.tasks.find((t) => t.id === id));
/** GET /v1/tasks/{id}/events */
export const useEvents = (id: string | undefined) => useStore((s) => (id ? s.events[id] : undefined));
/** GET /v1/campaigns */
export const useCampaigns = () => useStore((s) => s.campaigns);
/** GET /v1/campaigns/{id} */
export const useCampaign = (id: string | undefined) => useStore((s) => s.campaigns.find((c) => c.id === id));
/** GET /v1/control */
export const usePaused = () => useStore((s) => s.paused);
/** The decisions people have made, newest first. Read from the event record. */
export const useDecisions = () => useStore((s) => s.decisions);
/** GET /v1/artifacts/{id} and /content */
export const getArtifact = (id: string | undefined): Artifact | undefined => ARTIFACTS.find((a) => a.id === id);
/** GET /v1/config */
export const getConfig = (): Config => CONFIG;

/** Tasks waiting for a person, oldest first. */
export const usePending = () => {
  const tasks = useTasks();
  return tasks
    .filter((t) => t.run.status === 'awaiting_approval' && t.run.pending)
    .sort((a, b) => a.run.pending!.askedAt.localeCompare(b.run.pending!.askedAt));
};

// ---------- the budget, summed from the rows ----------

const round2 = (n: number) => Math.round(n * 100) / 100;
const TODAY = NOW.slice(0, 10);

export interface DaySpend { date: string; spent: number }

/** Inference spend per day this month. Today is summed from today's tasks. */
export function inferenceByDay(tasks: Task[] = state.tasks): DaySpend[] {
  const today = round2(tasks.filter((t) => t.createdAt.startsWith(TODAY)).reduce((a, t) => a + t.run.spent, 0));
  return INFERENCE_BY_DAY.map((d) => (d.date === TODAY ? { ...d, spent: today } : d));
}

/** GET /v1/budget */
export function getBudget(tasks: Task[] = state.tasks): Budget {
  const days = inferenceByDay(tasks);
  const day = days.find((d) => d.date === TODAY)?.spent ?? 0;
  const month = round2(days.reduce((a, d) => a + d.spent, 0));
  const searches = SEARCHES_BEFORE + tasks.reduce((a, t) => a + t.run.searches, 0);
  return {
    currency: 'INR',
    alertThresholds: [0.7, 0.9],
    limits: [
      { category: 'inference', period: 'day', limit: 150, spent: day },
      { category: 'inference', period: 'month', limit: 3000, spent: month },
      { category: 'research', period: 'month', limit: 500, spent: round2(searches * SEARCH_COST) },
    ],
  };
}
export const useBudget = () => {
  const tasks = useTasks();
  return getBudget(tasks);
};
export const searchesThisMonth = (tasks: Task[] = state.tasks) => SEARCHES_BEFORE + tasks.reduce((a, t) => a + t.run.searches, 0);

/** Inference spend this month by role, largest first. Only tasks in the sample are counted. */
export function spendByRole(tasks: Task[] = state.tasks): { role: string; label: string; tasks: number; spent: number }[] {
  return ROLES.map((r) => {
    const mine = tasks.filter((t) => t.role === r.name);
    return { role: r.name, label: r.label, tasks: mine.length, spent: round2(mine.reduce((a, t) => a + t.run.spent, 0)) };
  }).sort((a, b) => b.spent - a.spent);
}

// ---------- writes ----------

/** POST /v1/tasks/{id}/decision  { seq, approve, reason } */
export function decide(taskId: string, seq: number, approve: boolean, reason: string): void {
  const task = state.tasks.find((t) => t.id === taskId);
  const pending = task?.run.pending;
  if (!task || !pending || pending.seq !== seq) return; // the API answers 409 here
  const at = NOW;
  const tasks = state.tasks.map((t) =>
    t.id !== taskId
      ? t
      : {
          ...t,
          run: {
            ...t.run,
            pending: undefined,
            status: 'running' as const,
            reason: undefined,
            cap: approve && pending.action === 'budget.increase' ? Number(pending.input.to) : t.run.cap,
          },
        },
  );
  const campaigns = state.campaigns.map((c) =>
    c.id !== task.campaignId
      ? c
      : {
          ...c,
          status: 'running' as const,
          reason: undefined,
          updatedAt: at,
          steps: c.steps.map((s) => (s.taskId === taskId ? { ...s, status: 'running' as const, reason: undefined } : s)),
        },
  );
  const decision: Decision = { taskId, seq, action: pending.action, role: task.role, approve, reason, by: 'operator', at };
  const log: TaskEvent[] = [
    ...(state.events[taskId] ?? []),
    {
      at, type: approve ? 'Approved' : 'Rejected', tone: approve ? 'passed' : 'refused',
      detail: `By operator${reason ? `: “${reason}”` : ''}. ${approve ? 'The approval is used up by this one action.' : 'The model is told why, and continues inside the old limit.'}`,
    },
  ];
  set({ tasks, campaigns, decisions: [decision, ...state.decisions], events: { ...state.events, [taskId]: log } });
}

/** POST /v1/control  { paused } */
export function setPaused(paused: boolean): void {
  set({ paused });
}
