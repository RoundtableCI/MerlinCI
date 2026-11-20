import { NOW, ROLES } from '../data/sample';

const inr = new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const int = new Intl.NumberFormat('en-IN');

/** ₹1,550.86. Always two decimals, no space after the sign. */
export const money = (n: number) => `₹${inr.format(n)}`;
export const count = (n: number) => int.format(n);
export const percent = (ratio: number) => `${Math.round(ratio * 100)}%`;

export const roleLabel = (name: string | undefined) =>
  name === 'brand-guardian' ? 'Brand Guardian' : ROLES.find((r) => r.name === name)?.label ?? name ?? '';

/** task_4be19c07a2d1 → task_4be19c… */
export const shortId = (id: string) => (id.length > 13 ? `${id.slice(0, 12)}…` : id);

const pad = (n: number) => String(n).padStart(2, '0');
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/** 14:31 */
export const clock = (iso: string) => iso.slice(11, 16);
/** 19 Nov */
export const dayMonth = (iso: string) => `${Number(iso.slice(8, 10))} ${MONTHS[Number(iso.slice(5, 7)) - 1]}`;
/** Today 14:31, Yesterday 10:02, or 17 Nov 16:11. Measured against the sample's "now". */
export function when(iso: string): string {
  const days = Math.round((Date.parse(NOW.slice(0, 10)) - Date.parse(iso.slice(0, 10))) / 86_400_000);
  const day = days === 0 ? 'Today' : days === 1 ? 'Yesterday' : dayMonth(iso);
  return `${day} ${clock(iso)}`;
}
/** 1 h 11 m, 9 m, 42 s */
export function duration(fromIso: string, toIso: string = NOW): string {
  const s = Math.max(0, Math.round((Date.parse(toIso) - Date.parse(fromIso)) / 1000));
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} m ${pad(s % 60)} s`;
  return `${Math.floor(m / 60)} h ${pad(m % 60)} m`;
}
