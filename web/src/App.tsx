import { NavLink, Outlet, useLocation } from 'react-router-dom';

const LINKS = [
  { to: '/', label: 'Обзор', end: true },
  { to: '/dashboard', label: 'Дашборд', end: false },
  { to: '/transactions', label: 'Операции', end: false },
  { to: '/statements', label: 'Выписки', end: false },
  { to: '/receipts', label: 'Чеки', end: false },
  { to: '/categories', label: 'Категории', end: false },
];

export default function App() {
  // The selected period lives in the query string; carrying it across
  // pages keeps the dashboard and the operations list on the same slice.
  const { search } = useLocation();
  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">Расходы</div>
        <nav aria-label="Разделы">
          {LINKS.map((l) => (
            <NavLink
              key={l.to}
              to={{ pathname: l.to, search }}
              end={l.end}
              className={({ isActive }) => (isActive ? 'is-active' : undefined)}
            >
              {l.label}
            </NavLink>
          ))}
        </nav>
      </header>
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
