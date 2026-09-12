import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';

import App from './App';
import './index.css';
import Categories from './pages/Categories';
import Dashboard from './pages/Dashboard';
import Overview from './pages/Overview';
import Receipts from './pages/Receipts';
import Statements from './pages/Statements';
import Transactions from './pages/Transactions';

const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'dashboard', element: <Dashboard /> },
      { path: 'transactions', element: <Transactions /> },
      { path: 'statements', element: <Statements /> },
      { path: 'receipts', element: <Receipts /> },
      { path: 'categories', element: <Categories /> },
    ],
  },
]);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
