// The autonomy policy, mirrored from configs/examples/roundtableci/policy.yaml and
// internal/config/floors.go so the pages can show real verdicts without a server.
// When the console is connected, GET /v1/config should supply these instead.

export type Tier = 'automatic' | 'conditional' | 'explicit';

export interface Rule {
  action: string;
  tier: Tier;
  reason: string;
}

export const RULES: Rule[] = [
  { action: 'research.*', tier: 'automatic', reason: 'Research and competitor monitoring' },
  { action: 'draft.*', tier: 'automatic', reason: 'Drafting' },
  { action: 'campaign.plan', tier: 'automatic', reason: 'Campaign planning' },
  { action: 'seo.audit', tier: 'automatic', reason: 'SEO audits' },
  { action: 'analytics.internal.*', tier: 'automatic', reason: 'Internal analytics' },
  { action: 'schedule.internal', tier: 'automatic', reason: 'Internal scheduling' },
  { action: 'task.delegate', tier: 'automatic', reason: 'Delegation' },
  { action: 'publish.*', tier: 'explicit', reason: 'Publishing routine approved content' },
  { action: 'newsletter.schedule', tier: 'explicit', reason: 'Scheduling newsletters' },
  { action: 'site.update', tier: 'explicit', reason: 'Updating approved site content' },
  { action: 'reply.public', tier: 'explicit', reason: 'Public replies through authorized workflows' },
  { action: 'spend.*', tier: 'explicit', reason: 'Paid ad spend' },
  { action: 'budget.increase', tier: 'explicit', reason: 'Budget increases' },
  { action: 'outreach.*', tier: 'explicit', reason: 'Cold outreach and unsolicited DMs' },
  { action: 'list.import', tier: 'explicit', reason: 'Unapproved lists' },
  { action: 'campaign.delete', tier: 'explicit', reason: 'Deleting campaigns' },
];

export interface Floor {
  pattern: string;
  reason: string;
}

// Fixed in code: no policy file can lower these.
export const FLOORS: Floor[] = [
  { pattern: 'spend.*', reason: 'No autonomous ad spend' },
  { pattern: 'budget.increase.*', reason: 'Agents can never raise their own limit' },
  { pattern: 'outreach.*', reason: 'No autonomous cold outreach or unsolicited messages' },
];

// Actions that put something in front of people outside the company.
// Refused outright while the product profile is a draft.
export const PUBLIC_PREFIXES = ['publish', 'newsletter', 'site', 'reply', 'outreach'];

export function matchAction(pattern: string, action: string): boolean {
  if (pattern.endsWith('.*')) {
    const base = pattern.slice(0, -2);
    return action === base || action.startsWith(base + '.');
  }
  return pattern === action;
}

export function isPublic(action: string): boolean {
  return PUBLIC_PREFIXES.some((p) => action === p || action.startsWith(p + '.'));
}

export type Outcome = 'runs' | 'waits' | 'denied' | 'refused';

export interface Verdict {
  action: string;
  outcome: Outcome;
  tier?: Tier;
  rule?: Rule;
  floor?: Floor;
  /** One short phrase: what MerlinCI does with the proposal. */
  headline: string;
  /** One sentence: why. */
  detail: string;
}

export function decide(raw: string, opts: { profileApproved?: boolean } = {}): Verdict {
  const action = raw.trim().toLowerCase().replace(/\s+/g, '.');
  const rule = RULES.find((r) => matchAction(r.action, action));
  const floor = FLOORS.find((f) => matchAction(f.pattern, action));

  if (!action) {
    return { action, outcome: 'denied', headline: 'Nothing proposed', detail: 'Type an action such as publish.post.' };
  }
  if (opts.profileApproved === false && isPublic(action)) {
    return {
      action, outcome: 'refused', rule, floor, tier: rule?.tier,
      headline: 'Refused',
      detail: 'The product profile is still a draft, so every public-facing action is refused.',
    };
  }
  if (floor) {
    return {
      action, outcome: 'waits', tier: 'explicit', rule, floor,
      headline: 'Waits for a person',
      detail: `${floor.reason}. This is fixed in code; no policy file can change it.`,
    };
  }
  if (!rule) {
    return {
      action, outcome: 'denied',
      headline: 'Denied',
      detail: 'No rule covers this action, and an action no rule covers does not run.',
    };
  }
  if (rule.tier === 'automatic') {
    return {
      action, outcome: 'runs', tier: rule.tier, rule,
      headline: 'Runs',
      detail: `${rule.reason} is in the automatic tier. The call is recorded before and after it runs.`,
    };
  }
  return {
    action, outcome: 'waits', tier: rule.tier, rule,
    headline: 'Waits for a person',
    detail: `${rule.reason} needs a person's approval each time. One approval covers one action.`,
  };
}
