import { StrictMode, Suspense, lazy } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Route, Routes } from 'react-router-dom';

// The landing page and the console are separate bundles: neither loads the other's code or styles.
const Landing = lazy(() => import('./landing/Landing'));
const Shell = lazy(() => import('./console/Shell'));
const Overview = lazy(() => import('./console/Overview'));
const Campaigns = lazy(() => import('./console/Campaigns'));
const CampaignDetail = lazy(() => import('./console/CampaignDetail'));
const Approvals = lazy(() => import('./console/Approvals'));
const Budget = lazy(() => import('./console/Budget'));
const Rules = lazy(() => import('./console/Rules'));

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Suspense fallback={null}>
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/app" element={<Shell />}>
            <Route index element={<Overview />} />
            <Route path="campaigns" element={<Campaigns />} />
            <Route path="campaigns/:id" element={<CampaignDetail />} />
            <Route path="approvals" element={<Approvals />} />
            <Route path="budget" element={<Budget />} />
            <Route path="rules" element={<Rules />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  </StrictMode>,
);
