import { useEffect, useState } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom';
import { Wordmark } from '../components/Mark';
import { IS_SAMPLE, setPaused, usePaused, usePending } from '../data/api';
import { Icon } from './parts';
import '../styles/app-kit.css';
import '../styles/tokens.css';
import '../styles/console.css';

const NAV = [
  { to: '/app', label: 'Overview', icon: 'overview', end: true },
  { to: '/app/campaigns', label: 'Campaigns', icon: 'campaigns' },
  { to: '/app/approvals', label: 'Approvals', icon: 'approvals' },
  { to: '/app/budget', label: 'Budget', icon: 'budget' },
  { to: '/app/rules', label: 'Rules', icon: 'rules' },
];

export default function Shell() {
  const [open, setOpen] = useState(false);
  const paused = usePaused();
  const pending = usePending();
  const { pathname } = useLocation();
  const here = NAV.slice().reverse().find((n) => pathname.startsWith(n.to))?.label ?? 'Overview';

  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    document.documentElement.dataset.surface = 'console';
    document.title = `${here} · MerlinCI`;
  }, [here]);

  return (
    <div className="console app">
      <div className="topbar">
        <strong>{here}</strong>
        <button type="button" aria-label="Open the menu" onClick={() => setOpen(true)}><Icon name="menu" /></button>
      </div>

      <aside className={`sidebar ${open ? 'open' : ''}`}>
        <Link to="/app" className="brand" aria-label="MerlinCI console"><Wordmark size={18} /></Link>
        <button type="button" className="close" aria-label="Close the menu" onClick={() => setOpen(false)}><Icon name="close" /></button>
        <div className="scope">
          <span>Workspace</span>
          <span>RoundtableCI</span>
        </div>

        <nav aria-label="Console">
          {NAV.map((n) => (
            <NavLink key={n.to} to={n.to} end={n.end} className="nav-item">
              <Icon name={n.icon} />
              {n.label}
              {n.label === 'Approvals' && pending.length > 0 && (
                <span className="count" aria-label={`${pending.length} waiting`}>{pending.length}</span>
              )}
            </NavLink>
          ))}
        </nav>

        <div className="bottom">
          <Link to="/" className="nav-item" style={{ margin: '0 -8px' }}><Icon name="out" />merlinci site</Link>
          <label className="pause" data-paused={paused}>
            <span>
              <strong>{paused ? 'Every run is paused' : 'Pause every run'}</strong>
              <span>{paused ? 'Nothing moves until you resume.' : 'Each run stops where it is.'}</span>
            </span>
            <input className="switch" type="checkbox" checked={paused} onChange={(e) => setPaused(e.target.checked)} aria-label="Pause every run" />
          </label>
          {IS_SAMPLE && <p className="sample-note">Sample data. This console is not connected to a MerlinCI server yet.</p>}
          <div className="who"><i>O</i>operator, the one shared token</div>
        </div>
      </aside>

      <main className="canvas">
        {paused && (
          <div className="banner" role="status">
            <span><b>Every run is paused.</b> No model is called and no tool runs until you resume.</span>
            <button type="button" className="btn" onClick={() => setPaused(false)}>Resume</button>
          </div>
        )}
        <Outlet />
      </main>
    </div>
  );
}
