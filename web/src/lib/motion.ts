// Smooth scrolling (Lenis) wired to GSAP's ScrollTrigger, and the entrance every section shares.
import { useEffect, type RefObject } from 'react';
import Lenis from 'lenis';
import { gsap } from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

let lenisInstance: Lenis | null = null;
/** The page's Lenis instance, when smooth scrolling is on. */
export const getLenis = () => lenisInstance;

export const reducedMotion = () =>
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/** One Lenis instance for the page, driven by GSAP's clock so scroll and animation never drift. */
export function useSmoothScroll() {
  useEffect(() => {
    if (reducedMotion()) return;
    const lenis = new Lenis({ duration: 1.1, anchors: true });
    lenisInstance = lenis;
    lenis.on('scroll', ScrollTrigger.update);
    const tick = (t: number) => lenis.raf(t * 1000);
    gsap.ticker.add(tick);
    gsap.ticker.lagSmoothing(0);
    return () => {
      gsap.ticker.remove(tick);
      lenis.destroy();
      lenisInstance = null;
    };
  }, []);
}

/**
 * Entrance for the parts of each section: anything marked data-rise inside the root rises
 * 24px with opacity, 80ms apart, once, as it enters. Content is hidden only by this script,
 * so a page whose script fails still shows everything.
 */
export function useRise(root: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const el = root.current;
    if (!el || reducedMotion()) return;
    const ctx = gsap.context(() => {
      const groups = gsap.utils.toArray<HTMLElement>('[data-rise-group]');
      groups.forEach((group) => {
        const parts = group.querySelectorAll<HTMLElement>('[data-rise]');
        if (!parts.length) return;
        gsap.from(parts, {
          y: 24, opacity: 0, duration: 0.9, ease: 'power3.out', stagger: 0.08,
          scrollTrigger: { trigger: group, start: 'top 92%', once: true },
        });
      });
      // Pictures drift against the scroll: anything marked data-drift moves a little slower than the page.
      gsap.utils.toArray<HTMLElement>('[data-drift]').forEach((pic) => {
        gsap.fromTo(pic, { y: 70 }, { y: -70, ease: 'none', scrollTrigger: { trigger: pic, start: 'top bottom', end: 'bottom top', scrub: 0.6 } });
      });
    }, el);
    return () => ctx.revert();
  }, [root]);
}

export { gsap, ScrollTrigger };
