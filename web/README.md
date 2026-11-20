# web

The MerlinCI landing page and operator console: React, TypeScript and Vite. It is separate from
the Go backend and does not talk to it yet. Everything the console shows is sample data.

## Run it

```sh
cd web
npm install
npm run dev        # http://localhost:5173
npm run build      # type-check, then build into dist/
```

- `/` is the landing page.
- `/app` is the console: Overview, Campaigns, Approvals, Budget, Rules.

## Change how it looks

Edit `src/styles/tokens.css`. Section 1 is the brand (paper, ink, the teal accent, the signal
colour, the two typefaces). Sections 2 and 3 map the brand onto the landing page and the console.
The fonts (Gambarino, General Sans, JetBrains Mono for code) are loaded in `index.html`.

| To change | Look in |
|---|---|
| Colours, fonts, type sizes, margins, corner radius | `src/styles/tokens.css` |
| Landing page layout | `src/styles/landing.css` |
| Landing page words and sections | `src/story/MerlinStory.tsx`, `src/landing/sections.tsx` |
| Console layout | `src/styles/console.css` (on top of `app-kit.css`) |
| Console screens | `src/console/` |
| The logo mark | `src/components/Mark.tsx`, `public/merlin-mark.svg` |

## Motion

- The scroll story at the top of the landing page (`src/story/`): a sticky stage eight screens
  tall. Scroll position becomes one number and six cards (one brand's marketing) move through six scenes: found, made, held at the gate, measured, and the files a brand brings.
  `MerlinStory.tsx` holds the words, the scenes and where each role sits; `scroll-story.css`
  holds the look; `story-math.ts` and `useScrollStory.ts` are the engine.
  In development, `?p=0.48` freezes the story at a point and `?t=3` freezes time.
- Smooth scrolling: Lenis, driven by GSAP's clock (`src/lib/motion.ts`).
- Scroll animation: GSAP ScrollTrigger. The campaign bar pins and fills as you scroll.
- React Bits components, copied into `src/components/reactbits/`: SplitText (section headings),
  ScrollReveal (the closing headline), CountUp (the four figures), Magnet (the closing button).
- Everything is switched off for people who ask their system for reduced motion.

## Connect it to the backend later

`src/data/api.ts` is the only file the screens read data from. Each function names the API
endpoint it stands in for. Replace the bodies with `fetch` calls and delete `src/data/sample.ts`;
the screens do not change. `src/lib/policy.ts` mirrors `policy.yaml` and the floors by hand and
should then be filled from `GET /v1/config`.
