// SAMPLE DATA. Nothing here comes from a running MerlinCI. It is shaped like the API's answers
// so the console can be designed before it is connected; src/data/api.ts is the only file that
// reads it. Limits, caps, roles and policy are the real values from configs/. The campaigns,
// runs, amounts and sources are invented for the design and must not be quoted as results.
//
// The sample assumes the paid model list (the earlier placeholder in models.yaml), so that
// spend is visible. "Now" in the sample is 19 Nov 2026, 15:42 IST.

import type { Artifact, Campaign, CampaignStep, Config, Decision, RoleConfig, StepStatus, Task, TaskEvent } from './types';

export const NOW = '2026-11-19T15:42:00+05:30';
export const DIGEST = '9c41e7a2d05b';
export const SEARCH_COST = 0.5; // budget.yaml: unitCosts.research.search

const HAIKU = 'anthropic/claude-haiku-5.5';
const SONNET = 'anthropic/claude-sonnet-5.5';

export const ROLES: RoleConfig[] = [
  {
    name: 'market-researcher', label: 'Market Researcher', enabled: true, model: HAIKU,
    responsibility: 'Find public opportunities in demand, conversations, trends and competitors, and save the evidence for each.',
    tools: ['web-search', 'fetch-page'], permissions: ['research.search', 'research.fetch'],
    evaluators: ['evidence'], maxSteps: 12, maxCostPerTask: 25,
  },
  {
    name: 'competitor-analyst', label: 'Competitor Analyst', enabled: true, model: HAIKU,
    responsibility: 'Report what a competitor is doing in public, with the evidence for each observation.',
    tools: ['web-search', 'fetch-page'], permissions: ['research.search', 'research.fetch'],
    evaluators: ['evidence'], maxSteps: 12, maxCostPerTask: 25,
  },
  {
    name: 'campaign-strategist', label: 'Campaign Strategist', enabled: true, model: HAIKU,
    responsibility: 'Turn a researched opportunity into a campaign brief that a writer can act on.',
    tools: [], permissions: [], evaluators: ['evidence'], maxSteps: 4, maxCostPerTask: 10,
  },
  {
    name: 'content-writer', label: 'Content Writer', enabled: true, model: SONNET,
    responsibility: 'Write the content a campaign brief asks for, traced to its sources.',
    tools: ['fetch-page'], permissions: ['research.fetch'],
    evaluators: ['evidence', 'brand-guardian'], maxSteps: 8, maxCostPerTask: 40,
  },
  {
    name: 'distribution-manager', label: 'Distribution Manager', enabled: true, model: HAIKU,
    responsibility: 'Prepare an approved asset for publication, ready for a person to publish.',
    tools: [], permissions: [], evaluators: ['evidence', 'brand-guardian'], maxSteps: 4, maxCostPerTask: 15,
  },
  {
    name: 'campaign-analyst', label: 'Campaign Analyst', enabled: true, model: HAIKU,
    responsibility: 'Record what a campaign produced and how its results are to be measured.',
    tools: [], permissions: [], evaluators: ['evidence'], maxSteps: 4, maxCostPerTask: 10,
  },
];

export const roleOf = (name: string): RoleConfig => {
  const r = ROLES.find((x) => x.name === name);
  if (!r) throw new Error(`unknown role ${name}`);
  return r;
};

export const CONFIG: Config = {
  digest: DIGEST,
  profile: 'roundtableci',
  profileApproved: false, // the real profile is a draft: every public action is refused
  roles: ROLES,
};

// The first-release workflow, from configs/workflows/first-release.yaml.
export const WORKFLOW: { id: string; label: string; verb: string; role?: string; evaluator?: string; produces?: string }[] = [
  { id: 'find-opportunity', label: 'Find an opportunity', verb: 'Find', role: 'market-researcher', produces: 'market-opportunity' },
  { id: 'write-brief', label: 'Write the brief', verb: 'Brief', role: 'campaign-strategist', produces: 'campaign-brief' },
  { id: 'create-asset', label: 'Create the asset', verb: 'Write', role: 'content-writer', produces: 'content-asset' },
  { id: 'brand-check', label: 'Brand check', verb: 'Check', evaluator: 'brand-guardian' },
  { id: 'distribute', label: 'Distribute', verb: 'Prepare', role: 'distribution-manager', produces: 'publication' },
  { id: 'record-results', label: 'Record', verb: 'Record', role: 'campaign-analyst', produces: 'campaign-report' },
];

// ---------- tasks ----------

interface TaskSeed {
  id: string; role: string; topic: string; campaignId?: string;
  createdAt: string; endedAt?: string;
  status: Task['run']['status']; reason?: string;
  step: number; spent: number; inTok: number; outTok: number; sources: number; searches: number;
  artifactId?: string; pending?: Task['run']['pending'];
}

const seeds: TaskSeed[] = [
  // Campaign A, today: waiting on a person at step 3.
  { id: 'task_4be19c07a2d1', role: 'market-researcher', topic: 'comparing model outputs', campaignId: 'cmp_7f3a90c2e1b4',
    createdAt: '2026-11-19T13:58:10+05:30', endedAt: '2026-11-19T14:07:42+05:30', status: 'completed',
    step: 9, spent: 6.84, inTok: 41200, outTok: 3900, sources: 4, searches: 7, artifactId: 'art_c10d44e9' },
  { id: 'task_91c2f0aa37e5', role: 'campaign-strategist', topic: 'comparing model outputs', campaignId: 'cmp_7f3a90c2e1b4',
    createdAt: '2026-11-19T14:07:45+05:30', endedAt: '2026-11-19T14:10:03+05:30', status: 'completed',
    step: 2, spent: 1.92, inTok: 9800, outTok: 1650, sources: 4, searches: 0, artifactId: 'art_5a7e21b3' },
  { id: 'task_d3a8e6f41b90', role: 'content-writer', topic: 'comparing model outputs', campaignId: 'cmp_7f3a90c2e1b4',
    createdAt: '2026-11-19T14:10:06+05:30', status: 'awaiting_approval',
    reason: 'waiting for a person to decide on budget.increase',
    step: 5, spent: 31.4, inTok: 96400, outTok: 12100, sources: 4, searches: 0,
    pending: {
      seq: 1, tool: 'request-budget', action: 'budget.increase',
      input: { scope: 'this task', from: '40.00', to: '60.00', currency: 'INR' },
      rationale: 'Brand Guardian rejected two drafts for an unsupported comparison. One more revision is likely to cost about ₹12, which does not fit under the ₹40.00 cap.',
      askedAt: '2026-11-19T14:31:20+05:30',
    } },
  // A task someone started by hand, today.
  { id: 'task_0e5b7719cc42', role: 'competitor-analyst', topic: 'public pricing pages of model routers',
    createdAt: '2026-11-19T11:20:31+05:30', endedAt: '2026-11-19T11:32:08+05:30', status: 'completed',
    step: 10, spent: 8.15, inTok: 52300, outTok: 4100, sources: 5, searches: 5, artifactId: 'art_88b0f3d6' },

  // Campaign B, 18 Nov: completed end to end.
  { id: 'task_a71f3e2290bd', role: 'market-researcher', topic: 'cheaper models on agent tasks', campaignId: 'cmp_2c9e4d7710af',
    createdAt: '2026-11-18T10:02:11+05:30', endedAt: '2026-11-18T10:13:50+05:30', status: 'completed',
    step: 11, spent: 8.9, inTok: 58800, outTok: 4300, sources: 5, searches: 8, artifactId: 'art_1f6c9a20' },
  { id: 'task_b20c44d1e7f3', role: 'campaign-strategist', topic: 'cheaper models on agent tasks', campaignId: 'cmp_2c9e4d7710af',
    createdAt: '2026-11-18T10:13:53+05:30', endedAt: '2026-11-18T10:16:20+05:30', status: 'completed',
    step: 2, spent: 2.1, inTok: 10900, outTok: 1720, sources: 5, searches: 0, artifactId: 'art_2e7d0b31' },
  { id: 'task_c9d1e05a6b28', role: 'content-writer', topic: 'cheaper models on agent tasks', campaignId: 'cmp_2c9e4d7710af',
    createdAt: '2026-11-18T10:16:24+05:30', endedAt: '2026-11-18T10:29:47+05:30', status: 'completed',
    step: 4, spent: 27.65, inTok: 84100, outTok: 10800, sources: 5, searches: 0, artifactId: 'art_3a8e1c42' },
  { id: 'task_e4f20b9c1d35', role: 'distribution-manager', topic: 'cheaper models on agent tasks', campaignId: 'cmp_2c9e4d7710af',
    createdAt: '2026-11-18T10:31:02+05:30', endedAt: '2026-11-18T10:34:40+05:30', status: 'completed',
    step: 2, spent: 3.4, inTok: 17600, outTok: 2900, sources: 5, searches: 0, artifactId: 'art_4b9f2d53' },
  { id: 'task_f6a31c0d2e47', role: 'campaign-analyst', topic: 'cheaper models on agent tasks', campaignId: 'cmp_2c9e4d7710af',
    createdAt: '2026-11-18T10:34:44+05:30', endedAt: '2026-11-18T10:36:15+05:30', status: 'completed',
    step: 1, spent: 1.25, inTok: 8200, outTok: 900, sources: 5, searches: 0, artifactId: 'art_5c0a3e64' },

  // Campaign C, 17 Nov: Brand Guardian rejected the piece, so nothing after it ran.
  { id: 'task_17b8d92e4c60', role: 'market-researcher', topic: 'prompt routing for small teams', campaignId: 'cmp_e05b1a8834c9',
    createdAt: '2026-11-17T16:11:05+05:30', endedAt: '2026-11-17T16:21:33+05:30', status: 'completed',
    step: 10, spent: 7.55, inTok: 47900, outTok: 3600, sources: 3, searches: 9, artifactId: 'art_6d1b4f75' },
  { id: 'task_28c9ea3f5d71', role: 'campaign-strategist', topic: 'prompt routing for small teams', campaignId: 'cmp_e05b1a8834c9',
    createdAt: '2026-11-17T16:21:36+05:30', endedAt: '2026-11-17T16:23:58+05:30', status: 'completed',
    step: 2, spent: 1.8, inTok: 9100, outTok: 1500, sources: 3, searches: 0, artifactId: 'art_7e2c5086' },
  { id: 'task_39dafb4a6e82', role: 'content-writer', topic: 'prompt routing for small teams', campaignId: 'cmp_e05b1a8834c9',
    createdAt: '2026-11-17T16:24:01+05:30', endedAt: '2026-11-17T16:38:26+05:30', status: 'completed',
    step: 6, spent: 34.2, inTok: 104300, outTok: 13200, sources: 3, searches: 0, artifactId: 'art_8f3d6197' },

  // Campaign D, 16 Nov: research found no fetched source, so it failed at step 1.
  { id: 'task_4aeb0c5b7f93', role: 'market-researcher', topic: 'evals in CI for prompt changes', campaignId: 'cmp_91d6c20ff5e3',
    createdAt: '2026-11-16T12:40:18+05:30', endedAt: '2026-11-16T12:52:01+05:30', status: 'failed',
    reason: 'the evidence evaluator rejected the output three times: no fetched page supports the opportunity',
    step: 12, spent: 9.6, inTok: 63100, outTok: 4700, sources: 0, searches: 12 },
];

export const TASKS: Task[] = seeds.map((s) => {
  const role = roleOf(s.role);
  return {
    id: s.id, role: s.role, input: { topic: s.topic }, createdBy: 'operator',
    createdAt: s.createdAt, endedAt: s.endedAt, campaignId: s.campaignId,
    run: {
      status: s.status, reason: s.reason, step: s.step, maxSteps: role.maxSteps,
      spent: s.spent, cap: role.maxCostPerTask, currency: 'INR',
      inputTokens: s.inTok, outputTokens: s.outTok, sources: s.sources, searches: s.searches,
      artifactId: s.artifactId, configDigest: DIGEST, pending: s.pending,
    },
  };
});

// ---------- campaigns ----------

function steps(ids: (string | undefined)[], statuses: StepStatus[], reasons: Record<string, string> = {}): CampaignStep[] {
  return WORKFLOW.map((w, i) => {
    const task = ids[i] ? TASKS.find((t) => t.id === ids[i]) : undefined;
    return {
      id: w.id, label: w.label, role: w.role, evaluator: w.evaluator, produces: w.produces,
      status: statuses[i], taskId: task?.id, artifactId: task?.run.artifactId, reason: reasons[w.id],
    };
  });
}

export const CAMPAIGNS: Campaign[] = [
  {
    id: 'cmp_7f3a90c2e1b4', workflow: 'first-release', topic: 'Comparing model outputs', createdBy: 'operator',
    createdAt: '2026-11-19T13:58:08+05:30', updatedAt: '2026-11-19T14:31:20+05:30',
    status: 'awaiting_approval', reason: 'Content Writer is waiting for a decision on budget.increase',
    steps: steps(
      ['task_4be19c07a2d1', 'task_91c2f0aa37e5', 'task_d3a8e6f41b90'],
      ['completed', 'completed', 'awaiting_approval', 'pending', 'pending', 'pending'],
      { 'create-asset': 'Waiting for a person: budget.increase' },
    ),
  },
  {
    id: 'cmp_2c9e4d7710af', workflow: 'first-release', topic: 'Cheaper models on agent tasks', createdBy: 'operator',
    createdAt: '2026-11-18T10:02:09+05:30', updatedAt: '2026-11-18T10:36:15+05:30',
    status: 'completed',
    steps: steps(
      ['task_a71f3e2290bd', 'task_b20c44d1e7f3', 'task_c9d1e05a6b28', undefined, 'task_e4f20b9c1d35', 'task_f6a31c0d2e47'],
      ['completed', 'completed', 'completed', 'completed', 'completed', 'completed'],
    ),
  },
  {
    id: 'cmp_e05b1a8834c9', workflow: 'first-release', topic: 'Prompt routing for small teams', createdBy: 'operator',
    createdAt: '2026-11-17T16:11:03+05:30', updatedAt: '2026-11-17T16:39:02+05:30',
    status: 'stopped', reason: 'Brand Guardian rejected the piece',
    steps: steps(
      ['task_17b8d92e4c60', 'task_28c9ea3f5d71', 'task_39dafb4a6e82'],
      ['completed', 'completed', 'completed', 'rejected', 'skipped', 'skipped'],
      { 'brand-check': 'The piece calls routing “the fastest way to cut cost”. The profile approves no such claim.' },
    ),
  },
  {
    id: 'cmp_91d6c20ff5e3', workflow: 'first-release', topic: 'Evals in CI for prompt changes', createdBy: 'operator',
    createdAt: '2026-11-16T12:40:16+05:30', updatedAt: '2026-11-16T12:52:01+05:30',
    status: 'failed', reason: 'Market Researcher found no fetched source for an opportunity',
    steps: steps(
      ['task_4aeb0c5b7f93'],
      ['failed', 'skipped', 'skipped', 'skipped', 'skipped', 'skipped'],
      { 'find-opportunity': 'No fetched page supports the opportunity, so there is no opportunity.' },
    ),
  },
];

// ---------- artifacts (a few written out in full; the rest carry a title only) ----------

const SOURCES_A = [
  { url: 'https://news.ycombinator.com/item?id=41873310', title: 'Ask HN: How do you compare outputs across models?',
    retrievedAt: '2026-11-19T14:01:12+05:30', excerpt: 'We paste the same prompt into three tabs and eyeball it. There has to be a better way.' },
  { url: 'https://www.reddit.com/r/LocalLLaMA/comments/1gq2x7c/', title: 'Side-by-side evals without building a harness',
    retrievedAt: '2026-11-19T14:02:40+05:30', excerpt: 'Every harness I try wants me to write the grader first. I only want to see the answers next to each other.' },
  { url: 'https://simonwillison.net/2026/Oct/30/comparing-models/', title: 'Notes on comparing models by hand',
    retrievedAt: '2026-11-19T14:04:03+05:30', excerpt: 'The spreadsheet is still the most common eval tool I see in the wild.' },
  { url: 'https://github.com/openai/evals/discussions/1502', title: 'Lightweight comparison mode?',
    retrievedAt: '2026-11-19T14:05:27+05:30', excerpt: 'A request for a mode that runs one prompt across several models and prints the answers together.' },
];

export const ARTIFACTS: Artifact[] = [
  {
    id: 'art_c10d44e9', kind: 'market-opportunity', taskId: 'task_4be19c07a2d1', role: 'market-researcher',
    campaignId: 'cmp_7f3a90c2e1b4', title: 'People compare model outputs by hand, and say so',
    content:
      'What people are asking\nDevelopers describe pasting one prompt into several tabs and comparing the answers by eye. They ask for a way to see answers side by side without writing a grader first.\n\nWhy it matters\nThis is the first step of choosing a model, and it is being done in spreadsheets.\n\nConfidence\nMedium. Four public threads agree; none measures how common the practice is.',
    sources: SOURCES_A,
    evaluations: [{ evaluator: 'evidence', verdict: 'pass' }],
    cost: 6.84, createdAt: '2026-11-19T14:07:42+05:30',
  },
  {
    id: 'art_5a7e21b3', kind: 'campaign-brief', taskId: 'task_91c2f0aa37e5', role: 'campaign-strategist',
    campaignId: 'cmp_7f3a90c2e1b4', title: 'Brief: comparing model outputs without a harness',
    content:
      'Audience\nDevelopers choosing a model for one task.\n\nAngle\nA side-by-side read is a legitimate first eval. Show how to do it well.\n\nKey points\n1. Fix the prompt and the settings before comparing.\n2. Read answers blind where you can.\n3. Write down what "better" means for this task before reading.\n\nWhat not to say\nNothing about which model is best. Nothing about the product: the profile is a draft.',
    sources: SOURCES_A,
    evaluations: [{ evaluator: 'evidence', verdict: 'pass' }],
    cost: 1.92, createdAt: '2026-11-19T14:10:03+05:30',
  },
  {
    id: 'art_88b0f3d6', kind: 'competitor-brief', taskId: 'task_0e5b7719cc42', role: 'competitor-analyst',
    title: 'Public pricing pages of model routers, November 2026',
    content:
      'What they offer\nFive routers publish per-token prices with a routing margin on top. Two publish no margin and charge a flat platform fee.\n\nWhat is new\nOne added a free tier this month.\n\nWhat it means\nPricing is easy to compare; what each one routes on is not stated anywhere public.',
    sources: SOURCES_A.slice(0, 2).concat([
      { url: 'https://openrouter.ai/docs/faq', title: 'OpenRouter FAQ', retrievedAt: '2026-11-19T11:24:10+05:30', excerpt: 'How pricing is passed through from providers.' },
      { url: 'https://docs.litellm.ai/docs/routing', title: 'LiteLLM routing', retrievedAt: '2026-11-19T11:26:44+05:30', excerpt: 'Routing strategies and their settings.' },
      { url: 'https://portkey.ai/pricing', title: 'Portkey pricing', retrievedAt: '2026-11-19T11:28:02+05:30', excerpt: 'Plans and what each includes.' },
    ]),
    evaluations: [{ evaluator: 'evidence', verdict: 'pass' }],
    cost: 8.15, createdAt: '2026-11-19T11:32:08+05:30',
  },
  {
    id: 'art_3a8e1c42', kind: 'content-asset', taskId: 'task_c9d1e05a6b28', role: 'content-writer',
    campaignId: 'cmp_2c9e4d7710af', title: 'When a cheaper model is enough for an agent task',
    content:
      'Most agent tasks are not one hard problem. They are many small ones, and most of the small ones do not need the largest model.\n\nThis piece walks through three kinds of step where a smaller model does the job, and one kind where it does not.\n\nSources\nListed with the piece.',
    sources: SOURCES_A,
    evaluations: [{ evaluator: 'evidence', verdict: 'pass' }, { evaluator: 'brand-guardian', verdict: 'pass' }],
    cost: 27.65, createdAt: '2026-11-18T10:29:47+05:30',
  },
  {
    id: 'art_8f3d6197', kind: 'content-asset', taskId: 'task_39dafb4a6e82', role: 'content-writer',
    campaignId: 'cmp_e05b1a8834c9', title: 'Prompt routing for a team of five',
    content:
      'A small team rarely has time to test every model on every task. Routing is one way to stop choosing by habit.\n\n(Stored, then rejected at the independent brand check. Not distributed.)',
    sources: SOURCES_A.slice(0, 3),
    evaluations: [
      { evaluator: 'evidence', verdict: 'pass' },
      { evaluator: 'brand-guardian', verdict: 'reject', feedback: 'The piece calls routing “the fastest way to cut cost”. The profile approves no such claim.' },
    ],
    cost: 34.2, createdAt: '2026-11-17T16:38:26+05:30',
  },
];

// ---------- the event record of the run that is waiting ----------

export const EVENTS: Record<string, TaskEvent[]> = {
  task_d3a8e6f41b90: [
    { at: '2026-11-19T14:10:06+05:30', type: 'Task created', detail: 'By the campaign, for Content Writer. Cap ₹40.00, 8 model calls.' },
    { at: '2026-11-19T14:10:07+05:30', type: 'Budget reserved', detail: '₹9.80 held against today’s ₹150.00 before the first model call.' },
    { at: '2026-11-19T14:12:31+05:30', type: 'Tool proposed', detail: 'fetch-page on simonwillison.net. research.fetch is automatic.', tone: 'passed' },
    { at: '2026-11-19T14:12:33+05:30', type: 'Tool finished', detail: 'Page saved as a source. Its text reaches the model marked as untrusted data.' },
    { at: '2026-11-19T14:17:02+05:30', type: 'Draft evaluated', detail: 'Brand Guardian rejected draft 1: it says one model “wins”. The writer was told why.', tone: 'refused' },
    { at: '2026-11-19T14:24:48+05:30', type: 'Draft evaluated', detail: 'Brand Guardian rejected draft 2: a comparison with no source.', tone: 'refused' },
    { at: '2026-11-19T14:31:19+05:30', type: 'Tool proposed', detail: 'request-budget: raise this task’s cap from ₹40.00 to ₹60.00.' },
    { at: '2026-11-19T14:31:20+05:30', type: 'Run paused', detail: 'budget.increase always needs a person. Fixed in code as a floor.', tone: 'held' },
  ],
  task_4be19c07a2d1: [
    { at: '2026-11-19T13:58:10+05:30', type: 'Task created', detail: 'By the campaign, for Market Researcher. Cap ₹25.00, 12 model calls.' },
    { at: '2026-11-19T13:58:44+05:30', type: 'Tool proposed', detail: 'web-search “compare model outputs side by side”. research.search is automatic.', tone: 'passed' },
    { at: '2026-11-19T14:01:12+05:30', type: 'Tool finished', detail: 'fetch-page on news.ycombinator.com. Saved as source 1 of 4.' },
    { at: '2026-11-19T14:03:55+05:30', type: 'Tool proposed', detail: 'fetch-page on a private address. Refused: internal addresses are never read.', tone: 'refused' },
    { at: '2026-11-19T14:07:40+05:30', type: 'Output evaluated', detail: 'Evidence passed: every claim points at a fetched page.', tone: 'passed' },
    { at: '2026-11-19T14:07:42+05:30', type: 'Artifact stored', detail: 'Market opportunity, with 4 sources and its cost of ₹6.84.' },
  ],
};

// ---------- decisions already made (the record of who approved what) ----------

export const DECISIONS: Decision[] = [
  { taskId: 'task_c9d1e05a6b28', seq: 1, action: 'budget.increase', role: 'content-writer', approve: false,
    reason: 'Stay inside the cap; shorten the piece instead.', by: 'operator', at: '2026-11-18T10:24:10+05:30' },
  { taskId: 'task_17b8d92e4c60', seq: 1, action: 'list.import', role: 'market-researcher', approve: false,
    reason: 'That list was never approved.', by: 'operator', at: '2026-11-17T16:18:02+05:30' },
];

// ---------- spend before today ----------
// Inference spend per day for 1 to 18 Nov, in rupees. Days 17 and 18 are the sums of the
// tasks above; earlier days have no tasks in the sample and are given as totals.
const sumDay = (day: string, pick: (t: Task) => number) =>
  +TASKS.filter((t) => t.createdAt.startsWith(day)).reduce((a, t) => a + pick(t), 0).toFixed(2);

export const INFERENCE_BY_DAY: { date: string; spent: number }[] = [
  0, 0, 86.4, 112.25, 131.1, 97.8, 0, 0, 124.6, 141.35, 118.9, 149.2, 133.75, 0, 0,
].map((spent, i) => ({ date: `2026-11-${String(i + 1).padStart(2, '0')}`, spent }))
  .concat(['2026-11-16', '2026-11-17', '2026-11-18', '2026-11-19'].map((d) => ({ date: d, spent: sumDay(d, (t) => t.run.spent) })));

// Searches before the tasks in the sample (1 to 15 Nov).
export const SEARCHES_BEFORE = 214;
