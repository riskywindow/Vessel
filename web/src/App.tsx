import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Dashboard } from './pages/Dashboard';
import { AppsPage } from './pages/Apps';
import { AppDetailPage } from './pages/AppDetail';
import { HealthPage } from './pages/Health';
import { NetworkPage } from './pages/Network';
import { SecretsPage } from './pages/Secrets';
import { CertsPage } from './pages/Certs';
import { SettingsPage } from './pages/Settings';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 5000,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<Dashboard />} />
            <Route path="apps" element={<AppsPage />} />
            <Route path="apps/:name" element={<AppDetailPage />} />
            <Route path="health" element={<HealthPage />} />
            <Route path="network" element={<NetworkPage />} />
            <Route path="secrets" element={<SecretsPage />} />
            <Route path="certs" element={<CertsPage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
