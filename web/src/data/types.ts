// Shapes of what the MerlinCI API returns (internal/api/server.go), plus a few fields the
// console derives. Keep these in step with the Go structs when the two are connected.

export type RunStatus =
  | 'running'
  | 'awaiting_approval'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'pending';

export interface PendingView {
  seq: number;
  tool: string;
  /** The policy action the tool call maps to, such as budget.increase. */
  action: string;
  input: Record<string, unknown>;
  /** Why the model says it wants this. Shown to the person deciding. */
  rationale: string;
  askedAt: string;
}

export interface RunView {
  status: RunStatus;
  reason?: string;
  step: number;
  maxSteps: number;
  spent: number;
  cap: number;
  currency: 'INR';
  inputTokens: number;
  outputTokens: number;
  sources: number;
  searches: number;
  artifactId?: string;
  configDigest: string;
  pending?: PendingView;
}

export interface Task {
  id: string;
  role: string;
  input: Record<string, unknown>;
  createdBy: string;
  createdAt: string;
  endedAt?: string;
  campaignId?: string;
  run: RunView;
}

export interface TaskEvent {
  at: string;
  type: string;
  detail: string;
  /** Marks the events a person should notice: a refusal, a pause, a decision. */
  tone?: 'plain' | 'held' | 'refused' | 'passed';
}

export interface Source {
  url: string;
  title: string;
  retrievedAt: string;
  excerpt: string;
}

export interface Evaluation {
  evaluator: string;
  verdict: 'pass' | 'reject';
  feedback?: string;
}

export interface Artifact {
  id: string;
  kind: string;
  taskId: string;
  role: string;
  campaignId?: string;
  title: string;
  /** Model output, always shown as plain text. */
  content: string;
  sources: Source[];
  evaluations: Evaluation[];
  cost: number;
  createdAt: string;
}

export type StepStatus = 'completed' | 'running' | 'awaiting_approval' | 'rejected' | 'failed' | 'pending' | 'skipped';

export interface CampaignStep {
  id: string;
  label: string;
  role?: string;
  evaluator?: string;
  produces?: string;
  status: StepStatus;
  taskId?: string;
  artifactId?: string;
  reason?: string;
}

export type CampaignStatus = 'running' | 'awaiting_approval' | 'completed' | 'stopped' | 'failed' | 'cancelled';

export interface Campaign {
  id: string;
  workflow: string;
  topic: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  status: CampaignStatus;
  reason?: string;
  steps: CampaignStep[];
}

export interface BudgetLine {
  category: 'inference' | 'research';
  period: 'day' | 'month';
  limit: number;
  spent: number;
}

export interface Budget {
  currency: 'INR';
  alertThresholds: number[];
  limits: BudgetLine[];
}

export interface RoleConfig {
  name: string;
  label: string;
  enabled: boolean;
  model: string;
  responsibility: string;
  tools: string[];
  permissions: string[];
  evaluators: string[];
  maxSteps: number;
  maxCostPerTask: number;
}

export interface Config {
  digest: string;
  profile: string;
  profileApproved: boolean;
  roles: RoleConfig[];
}

export interface Decision {
  taskId: string;
  seq: number;
  action: string;
  role: string;
  approve: boolean;
  reason: string;
  by: string;
  at: string;
}
