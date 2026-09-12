// Typed client for the JSON API. Amounts are integers in kopecks; dates
// are YYYY-MM-DD strings; timestamps are RFC 3339 in the statement zone.

export type Granularity = 'day' | 'week' | 'month';
export type Direction = 'expense' | 'income';

export interface Statement {
  id: number;
  bank: string;
  number: string;
  period_from: string;
  period_to: string;
  opening_balance: number;
  closing_balance: number;
  total_income: number;
  total_expense: number;
  file_name: string;
  tx_count: number;
  uploaded_at: string;
}

export interface ImportResult {
  statement: Statement;
  imported: number;
  duplicates: number;
  warnings: string[];
}

export interface Transaction {
  id: number;
  statement_id: number;
  source: 'statement' | 'receipt';
  op_at: string;
  op_date: string;
  processed_on: string;
  card: string;
  amount: number;
  direction: Direction;
  kind: string;
  description: string;
  merchant: string;
  mcc: string;
  counterparty: string;
  category_id: number | null;
  category_source: string;
  note: string;
  excluded: boolean;
  receipt_id: number | null;
}

export interface Category {
  id: number;
  name: string;
  color: string;
  is_transfer: boolean;
  sort_order: number;
}

export interface Rule {
  id: number;
  category_id: number;
  field: string;
  op: string;
  value: string;
  priority: number;
}

export interface Bucket {
  start: string;
  expense: number;
  income: number;
  count: number;
}

export interface Totals {
  expense: number;
  income: number;
  count: number;
}

export interface Summary {
  from: string;
  to: string;
  expense: number;
  income: number;
  count: number;
  days: number;
  active_days: number;
  avg_per_day: number;
  avg_per_active_day: number;
  max_day: Bucket | null;
  prev: Totals;
}

export interface CategoryTotal {
  category_id: number | null;
  name: string;
  color: string;
  is_transfer: boolean;
  expense: number;
  income: number;
  count: number;
}

export interface MerchantTotal {
  merchant: string;
  expense: number;
  count: number;
}

export interface DataRange {
  from: string;
  to: string;
  has_data: boolean;
}

export interface Budget {
  id: number;
  category_id: number | null;
  amount: number;
}

export interface BudgetStatus {
  budget_id: number;
  category_id: number | null;
  name: string;
  color: string;
  limit: number;
  spent: number;
  forecast: number;
  pace_limit: number;
}

export interface BudgetReport {
  month: string;
  days_in_month: number;
  days_elapsed: number;
  items: BudgetStatus[];
  total: BudgetStatus;
}

export interface Period {
  from: string;
  to: string;
}

export interface CategoryDelta {
  category_id: number | null;
  name: string;
  color: string;
  is_transfer: boolean;
  current: number;
  previous: number;
}

export interface MerchantDelta {
  merchant: string;
  current: number;
  previous: number;
}

export interface Comparison {
  current: Period;
  previous: Period;
  current_totals: Totals;
  previous_totals: Totals;
  categories: CategoryDelta[];
  merchants: MerchantDelta[];
}

export interface TransactionPatch {
  category_id?: number | null;
  note?: string;
  excluded?: boolean;
}

export interface ReceiptSession {
  configured: boolean;
  phone: string;
  device_id: string;
  token_expires_at: string;
  refresh_expires_at: string;
  updated_at: string;
}

export interface ReceiptSessionInput {
  phone: string;
  device_id: string;
  refresh_token: string;
  access_token: string;
}

export interface ReceiptSyncStatus {
  running: boolean;
  started_at: string;
  finished_at: string;
  listed: number;
  added: number;
  detailed: number;
  pending: number;
  matched: number;
  error: string;
  next_run_at: string;
}

export interface ReceiptItem {
  position: number;
  name: string;
  price: number;
  quantity: number;
  sum: number;
  category_id: number | null;
}

export interface Receipt {
  id: number;
  key: string;
  seller_name: string;
  seller_inn: string;
  retail_place: string;
  retail_address: string;
  issued_at: string;
  issued_on: string;
  received_at: string;
  operation_type: number;
  total: number;
  cash_total: number;
  ecash_total: number;
  items_loaded: boolean;
  items_error: string;
  transaction_id: number | null;
  match_kind: string;
  item_count: number;
  items?: ReceiptItem[];
}

export interface ReceiptParams {
  from?: string;
  to?: string;
  unmatched?: boolean;
  limit?: number;
  offset?: number;
}

export interface ItemTotal {
  name: string;
  quantity: number;
  sum: number;
  count: number;
}

export interface ListResponse<T> {
  items: T[];
  total: number;
}

/** Operations list: the sums cover every row the filter matches, not the page. */
export interface TransactionList extends ListResponse<Transaction> {
  expense: number;
  income: number;
}

export interface Signal {
  kind: string;
  severity: 'bad' | 'warn' | 'info';
  title: string;
  detail: string;
  link: string;
}

export interface Overview {
  today: string;
  month: {
    month: string;
    days_elapsed: number;
    days_in_month: number;
    spent: number;
    forecast: number;
    limit: number;
    pace_limit: number;
    prev_same_point: number;
    prev_total: number;
  };
  week: { from: string; to: string; spent: number; days: Bucket[]; typical_week: number; weeks_in_avg: number };
  trend: { months: Bucket[]; average: number };
  signals: Signal[];
  recent: Transaction[];
}

export interface RangeParams {
  from: string;
  to: string;
  include_transfers?: boolean;
}

export interface TransactionParams extends RangeParams {
  id?: number;
  category_id?: number;
  uncategorized?: boolean;
  direction?: Direction | '';
  q?: string;
  limit?: number;
  offset?: number;
}

export interface CategoryInput {
  name: string;
  color: string;
  is_transfer: boolean;
  sort_order: number;
}

export interface RuleInput {
  category_id: number;
  field: string;
  op: string;
  value: string;
  priority: number;
}

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, message: string, code: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

// Plain-language messages for the errors a person can act on; anything
// else falls back to the server text.
const FRIENDLY: Record<string, string> = {
  receipt_session: 'Ключ доступа к «Мои чеки онлайн» не подключён или устарел. Обновите его в разделе «Чеки».',
  too_large: 'Файл слишком большой.',
};

export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    if (FRIENDLY[e.code]) return FRIENDLY[e.code];
    if (e.status === 409 && e.message.includes('sync is already running')) return 'Загрузка чеков уже идёт.';
    if (e.message.includes('session check failed')) return 'Ключ не подошёл. Проверьте, что скопирован весь текст, и попробуйте снова.';
    if (e.status >= 500) return 'Что-то пошло не так. Попробуйте ещё раз.';
    return e.message.replace(/^invalid input: /, '');
  }
  if (e instanceof Error) return e.message;
  return String(e);
}

// Builds a query string, dropping empty and false values so optional
// filters are only sent when they are set.
function qs(params: object): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params) as Array<[string, unknown]>) {
    if (v === undefined || v === null || v === '' || v === false) continue;
    sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : '';
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (res.status === 204) return undefined as T;
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  if (!res.ok) {
    const err = (body ?? {}) as { error?: string; code?: string };
    throw new ApiError(res.status, err.error ?? `${res.status} ${res.statusText}`, err.code ?? 'unknown');
  }
  return body as T;
}

function json(method: string, body: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
}

export const api = {
  overview: () => request<Overview>('/api/v1/overview'),
  statements: {
    list: () => request<ListResponse<Statement>>('/api/v1/statements'),
    upload: (file: File) => {
      const fd = new FormData();
      fd.append('file', file, file.name);
      return request<ImportResult>('/api/v1/statements', { method: 'POST', body: fd });
    },
    remove: (id: number) => request<void>(`/api/v1/statements/${id}`, { method: 'DELETE' }),
  },
  transactions: {
    list: (p: TransactionParams) => request<TransactionList>(`/api/v1/transactions${qs(p)}`),
    patch: (id: number, body: TransactionPatch) =>
      request<Transaction>(`/api/v1/transactions/${id}`, json('PATCH', body)),
  },
  categories: {
    list: () => request<ListResponse<Category>>('/api/v1/categories'),
    create: (c: CategoryInput) => request<Category>('/api/v1/categories', json('POST', c)),
    update: (id: number, c: CategoryInput) => request<Category>(`/api/v1/categories/${id}`, json('PUT', c)),
    remove: (id: number) => request<void>(`/api/v1/categories/${id}`, { method: 'DELETE' }),
  },
  rules: {
    list: () => request<ListResponse<Rule>>('/api/v1/rules'),
    create: (r: RuleInput) => request<Rule>('/api/v1/rules', json('POST', r)),
    remove: (id: number) => request<void>(`/api/v1/rules/${id}`, { method: 'DELETE' }),
    apply: () => request<{ updated: number }>('/api/v1/rules/apply', { method: 'POST' }),
  },
  analytics: {
    summary: (p: RangeParams) => request<Summary>(`/api/v1/analytics/summary${qs(p)}`),
    series: (p: RangeParams, granularity: Granularity) =>
      request<{ granularity: Granularity; items: Bucket[] }>(`/api/v1/analytics/series${qs({ ...p, granularity })}`),
    categories: (p: RangeParams) => request<ListResponse<CategoryTotal>>(`/api/v1/analytics/categories${qs(p)}`),
    merchants: (p: RangeParams, limit: number) =>
      request<ListResponse<MerchantTotal>>(`/api/v1/analytics/merchants${qs({ ...p, limit })}`),
    compare: (p: RangeParams) => request<Comparison>(`/api/v1/analytics/compare${qs(p)}`),
    items: (p: RangeParams, limit: number) =>
      request<ListResponse<ItemTotal>>(`/api/v1/analytics/items${qs({ ...p, limit })}`),
    range: () => request<DataRange>('/api/v1/analytics/range'),
  },
  receipts: {
    session: () => request<ReceiptSession>('/api/v1/receipts/session'),
    setSession: (s: ReceiptSessionInput) => request<ReceiptSession>('/api/v1/receipts/session', json('PUT', s)),
    clearSession: () => request<void>('/api/v1/receipts/session', { method: 'DELETE' }),
    syncStatus: () => request<ReceiptSyncStatus>('/api/v1/receipts/sync'),
    startSync: () => request<ReceiptSyncStatus>('/api/v1/receipts/sync', { method: 'POST' }),
    match: () => request<{ matched: number }>('/api/v1/receipts/match', { method: 'POST' }),
    list: (p: ReceiptParams) => request<ListResponse<Receipt>>(`/api/v1/receipts${qs(p)}`),
    get: (id: number) => request<Receipt>(`/api/v1/receipts/${id}`),
    candidates: (id: number) => request<ListResponse<Transaction>>(`/api/v1/receipts/${id}/candidates`),
    link: (id: number, transaction_id: number) =>
      request<Receipt>(`/api/v1/receipts/${id}/link`, json('POST', { transaction_id })),
    unlink: (id: number) => request<void>(`/api/v1/receipts/${id}/link`, { method: 'DELETE' }),
    forTransaction: (txId: number) => request<ListResponse<Receipt>>(`/api/v1/transactions/${txId}/receipts`),
  },
  budgets: {
    list: () => request<ListResponse<Budget>>('/api/v1/budgets'),
    set: (category_id: number | null, amount: number) =>
      request<Budget>('/api/v1/budgets', json('PUT', { category_id, amount })),
    remove: (id: number) => request<void>(`/api/v1/budgets/${id}`, { method: 'DELETE' }),
    report: (month: string) => request<BudgetReport>(`/api/v1/budgets/report${qs({ month })}`),
  },
};
