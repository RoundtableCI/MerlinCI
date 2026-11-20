import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Wordmark } from '../components/Mark';
import { useRise, useSmoothScroll } from '../lib/motion';
import { MerlinStory } from '../story/MerlinStory';
import { Ask, CampaignBar, Close, Foot, Never, Parts, Record, Status } from './sections';
import '../styles/tokens.css';
import '../styles/landing.css';

export default function Landing() {
  const root = useRef<HTMLDivElement>(null);
  // One switch pauses the ambient motion of the scroll story; it also lives in the paragraph further down.
  const [paused, setPaused] = useState(false);

  useSmoothScroll();
  useRise(root);
  useEffect(() => {
    document.documentElement.dataset.surface = 'site';
    document.title = 'MerlinCI: marketing that shows its work';
  }, []);

  return (
    <div className="site" id="top" ref={root}>
      <div className="top wrap">
        <Link to="/" aria-label="MerlinCI, home"><Wordmark size={19} /></Link>
        <nav aria-label="Sections">
          <a className="link wide" href="#campaign">How a campaign runs</a>
          <a className="link wide" href="#policy">Policy</a>
          <a className="link wide" href="#status">Status</a>
          <Link className="link" to="/app">Console</Link>
        </nav>
      </div>
      <MerlinStory paused={paused} onPause={setPaused} />
      <main>
        <Record />
        <CampaignBar />
        <Parts paused={paused} onPause={setPaused} />
        <Ask />
        <Never />
        <Status />
        <Close />
      </main>
      <Foot />
    </div>
  );
}
