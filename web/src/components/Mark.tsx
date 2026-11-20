// The MerlinCI mark: a round table with one seat left open, and a person in it.
// The ring is the family tie to RoundtableCI; the dot in the gap is what MerlinCI adds.
export function Mark({ size = 20, title }: { size?: number; title?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" role={title ? 'img' : undefined} aria-hidden={title ? undefined : true} aria-label={title}>
      <path d="M17.2 4.6A9.5 9.5 0 1 0 21.3 10" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" />
      <circle cx="20.2" cy="5.6" r="2.6" fill="var(--brand-teal)" />
    </svg>
  );
}

export function Wordmark({ size = 20 }: { size?: number }) {
  return (
    <span className="wordmark" style={{ display: 'inline-flex', alignItems: 'center', gap: size * 0.4 }}>
      <Mark size={size} />
      <span style={{ fontFamily: 'var(--font-display)', fontSize: size * 1.1, lineHeight: 1, letterSpacing: '-0.01em' }}>MerlinCI</span>
    </span>
  );
}
