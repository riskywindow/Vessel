import { Link, Outlet, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Box,
  Activity,
  Settings,
  Key,
  Network,
  Shield,
} from 'lucide-react';
import { cn } from '../lib/utils';

const navigation = [
  { name: 'Dashboard', href: '/', icon: LayoutDashboard },
  { name: 'Apps', href: '/apps', icon: Box },
  { name: 'Health', href: '/health', icon: Activity },
  { name: 'Network', href: '/network', icon: Network },
  { name: 'Secrets', href: '/secrets', icon: Key },
  { name: 'Certificates', href: '/certs', icon: Shield },
  { name: 'Settings', href: '/settings', icon: Settings },
];

export function Layout() {
  const location = useLocation();

  return (
    <div className="min-h-screen bg-gray-100">
      {/* Sidebar */}
      <div className="fixed inset-y-0 left-0 w-64 bg-gray-900">
        <div className="flex h-16 items-center px-6">
          <Link to="/" className="text-xl font-bold text-white">
            Vessel
          </Link>
        </div>
        <nav className="mt-6 px-3 space-y-1">
          {navigation.map((item) => {
            const isActive =
              location.pathname === item.href ||
              (item.href !== '/' && location.pathname.startsWith(item.href));
            return (
              <Link
                key={item.name}
                to={item.href}
                className={cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-gray-800 text-white'
                    : 'text-gray-400 hover:bg-gray-800 hover:text-white'
                )}
              >
                <item.icon className="h-5 w-5" />
                {item.name}
              </Link>
            );
          })}
        </nav>
      </div>

      {/* Main content */}
      <div className="pl-64">
        <main className="py-6 px-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
