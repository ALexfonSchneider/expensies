import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';

import {
  api,
  errorMessage,
  type Receipt,
  type ReceiptSession,
  type ReceiptSessionInput,
  type ReceiptSyncStatus,
  type Transaction,
} from '../api';
import ReceiptItems from '../components/ReceiptItems';
import { addDays, fmtDateTime, int, money, plural, shortSeller, signedMoney } from '../format';

// The operation may sit a day or two away from the fiscal time, so the
// jump opens a small window around the receipt and pins the single id.
function operationLink(r: Receipt): string {
  const sp = new URLSearchParams({
    preset: 'custom',
    from: addDays(r.issued_on, -2),
    to: addDays(r.issued_on, 2),
    id: String(r.transaction_id),
  });
  return `/transactions?${sp.toString()}`;
}

const EMPTY_INPUT: ReceiptSessionInput = { phone: '', device_id: '', refresh_token: '', access_token: '' };

interface CandidatePickerProps {
  receipt: Receipt;
  onLinked: () => void;
  onError: (e: unknown) => void;
}

function CandidatePicker({ receipt, onLinked, onError }: CandidatePickerProps) {
  const [cands, setCands] = useState<Transaction[] | null>(null);
  useEffect(() => {
    api.receipts
      .candidates(receipt.id)
      .then((r) => setCands(r.items))
      .catch(onError);
  }, [receipt.id, onError]);

  if (cands === null) return <div className="hint">Ищем операции на {money(receipt.total)}…</div>;
  if (cands.length === 0) {
    return <div className="hint">Операций на {money(receipt.total)} в пределах трёх дней от чека не нашлось.</div>;
  }
  return (
    <div className="candidates">
      {cands.map((t) => (
        <div key={t.id} className="candidate">
          <span className="nowrap">{fmtDateTime(t.op_at)}</span>
          <span className="ellipsis-inline">{t.merchant || t.counterparty || t.description}</span>
          <span className="num amount">{signedMoney(t.amount, t.direction)}</span>
          <button
            type="button"
            className="btn btn-small btn-primary"
            onClick={() => api.receipts.link(receipt.id, t.id).then(onLinked).catch(onError)}
          >
            Привязать
          </button>
        </div>
      ))}
    </div>
  );
}

export default function Receipts() {
  const [session, setSession] = useState<ReceiptSession | null>(null);
  const [form, setForm] = useState<ReceiptSessionInput>(EMPTY_INPUT);
  const [showForm, setShowForm] = useState(false);
  const [status, setStatus] = useState<ReceiptSyncStatus | null>(null);
  const [receipts, setReceipts] = useState<Receipt[]>([]);
  const [total, setTotal] = useState(0);
  const [unmatchedOnly, setUnmatchedOnly] = useState(true);
  const [open, setOpen] = useState<{ id: number; kind: 'items' | 'link' } | null>(null);
  const [details, setDetails] = useState<Record<number, Receipt>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const fail = useCallback((e: unknown) => setError(errorMessage(e)), []);

  const loadSession = useCallback(() => api.receipts.session().then(setSession).catch(fail), [fail]);
  const loadStatus = useCallback(() => api.receipts.syncStatus().then(setStatus).catch(fail), [fail]);
  const loadReceipts = useCallback(
    () =>
      api.receipts
        .list({ unmatched: unmatchedOnly, limit: 100 })
        .then((r) => {
          setReceipts(r.items);
          setTotal(r.total);
        })
        .catch(fail),
    [fail, unmatchedOnly],
  );

  useEffect(() => {
    void loadSession();
    void loadStatus();
  }, [loadSession, loadStatus]);

  useEffect(() => {
    void loadReceipts();
  }, [loadReceipts]);

  // While a sync runs, poll its progress and refresh the list at the end.
  useEffect(() => {
    if (!status?.running) return;
    const t = setInterval(() => {
      api.receipts
        .syncStatus()
        .then((s) => {
          setStatus(s);
          if (!s.running) void loadReceipts();
        })
        .catch(fail);
    }, 3000);
    return () => clearInterval(t);
  }, [status?.running, loadReceipts, fail]);

  const saveSession = (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    api.receipts
      .setSession(form)
      .then((s) => {
        setSession(s);
        setShowForm(false);
        setForm(EMPTY_INPUT);
        setNotice('Сессия проверена и сохранена. Теперь можно синхронизировать чеки.');
      })
      .catch(fail)
      .finally(() => setBusy(false));
  };

  const clearSession = () => {
    if (!window.confirm('Отключить архив чеков? Загруженные чеки останутся.')) return;
    api.receipts
      .clearSession()
      .then(loadSession)
      .catch(fail);
  };

  const matchNow = () => {
    setError(null);
    api.receipts
      .match()
      .then((r) => {
        setNotice(`Сопоставлено чеков: ${int(r.matched)}`);
        void loadReceipts();
      })
      .catch(fail);
  };

  const startSync = () => {
    setError(null);
    setNotice(null);
    api.receipts.startSync().then(setStatus).catch(fail);
  };

  const toggle = (r: Receipt, kind: 'items' | 'link') => {
    if (open && open.id === r.id && open.kind === kind) {
      setOpen(null);
      return;
    }
    setOpen({ id: r.id, kind });
    if (kind === 'items' && !details[r.id]) {
      api.receipts
        .get(r.id)
        .then((full) => setDetails((d) => ({ ...d, [r.id]: full })))
        .catch(fail);
    }
  };

  const unlink = (r: Receipt) => {
    api.receipts
      .unlink(r.id)
      .then(loadReceipts)
      .catch(fail);
  };

  return (
    <div className="page">
      <h1>Чеки</h1>
      {error && <div className="alert">{error}</div>}
      {notice && <div className="alert alert-ok">{notice}</div>}

      <div className="grid-2">
        <section className="card">
          <div className="card-head">
            <h2>Архив «Мои чеки онлайн»</h2>
            {session?.configured && (
              <span className="hint">
                {session.phone} · обновлено {fmtDateTime(session.updated_at)}
              </span>
            )}
          </div>
          {session?.configured && !showForm ? (
            <div className="inline-form">
              <button type="button" className="btn btn-primary" disabled={status?.running ?? false} onClick={startSync}>
                {status?.running ? 'Синхронизация идёт…' : 'Синхронизировать'}
              </button>
              <button type="button" className="btn" onClick={matchNow}>
                Сопоставить с операциями
              </button>
              <button type="button" className="btn" onClick={() => setShowForm(true)}>
                Обновить сессию
              </button>
              <button type="button" className="btn btn-danger" onClick={clearSession}>
                Отключить
              </button>
            </div>
          ) : (
            <form className="session-form" onSubmit={saveSession}>
              <label>
                Ответ <code>auth/token</code> целиком или только refresh token
                <textarea
                  value={form.refresh_token}
                  onChange={(e) => setForm({ ...form, refresh_token: e.target.value })}
                  rows={5}
                  placeholder='{"refreshToken": "...", "token": "...", ...}'
                  required
                />
              </label>
              <p className="hint">
                Телефон и device id читаются из самого refresh-токена, поля ниже нужны только если хотите их
                переопределить.
              </p>
              <label>
                Телефон
                <input
                  value={form.phone}
                  onChange={(e) => setForm({ ...form, phone: e.target.value })}
                  placeholder="+7 999 123-45-67"
                />
              </label>
              <label>
                Device ID
                <input value={form.device_id} onChange={(e) => setForm({ ...form, device_id: e.target.value })} />
              </label>
              <label>
                Access token <span className="hint">(необязательно, обновится сам)</span>
                <textarea
                  value={form.access_token}
                  onChange={(e) => setForm({ ...form, access_token: e.target.value })}
                  rows={2}
                />
              </label>
              <div className="inline-form">
                <button type="submit" className="btn btn-primary" disabled={busy}>
                  {busy ? 'Проверяем…' : 'Сохранить и проверить'}
                </button>
                {session?.configured && (
                  <button type="button" className="btn" onClick={() => setShowForm(false)}>
                    Отмена
                  </button>
                )}
              </div>
            </form>
          )}
          <details className="howto">
            <summary>Как получить токены</summary>
            <ol>
              <li>
                Войдите на <a href="https://lkdr.nalog.ru" target="_blank" rel="noreferrer">lkdr.nalog.ru</a> как обычно
                (капча и код из SMS).
              </li>
              <li>
                Откройте DevTools (F12), вкладка Network, фильтр <code>mco.nalog.ru</code>. Перезагрузите страницу.
              </li>
              <li>
                Найдите запрос <code>auth/token</code> (или <code>auth/challenge/sms/verify</code> сразу после входа) и
                скопируйте его ответ целиком, вкладка Response.
              </li>
              <li>
                Вставьте ответ в поле выше. Приложение само обновляет токены; когда refresh token перестанет приниматься,
                повторите вход и обновите сессию.
              </li>
            </ol>
            <p className="hint">
              Токены дают доступ ко всем вашим чекам и хранятся только в локальной базе. Вход с капчей приложение не
              выполняет и не обходит.
            </p>
          </details>
        </section>

        <section className="card">
          <div className="card-head">
            <h2>Синхронизация</h2>
          </div>
          {status ? (
            <div className="sync-status">
              {status.running ? (
                <p>
                  <strong>Идёт синхронизация</strong> с {fmtDateTime(status.started_at)}: просмотрено {int(status.listed)},
                  новых {int(status.added)}, с позициями {int(status.detailed)}, в очереди {int(status.pending)}.
                </p>
              ) : status.finished_at ? (
                <p>
                  Последняя: {fmtDateTime(status.finished_at)}. Просмотрено {int(status.listed)}, новых{' '}
                  {int(status.added)}, загружено позиций для {int(status.detailed)}, сопоставлено {int(status.matched)}
                  {status.pending > 0 && <>, ещё без позиций {int(status.pending)}</>}.
                </p>
              ) : (
                <p className="hint">Синхронизация ещё не запускалась.</p>
              )}
              {status.error && <div className="alert">{status.error}</div>}
              <p className="hint">
                Новые чеки подтягиваются сами каждые 6 часов и при старте сервера, страницу держать открытой не нужно.
                Архив отдаёт около 20 запросов в минуту, поэтому первая загрузка длинной истории занимает время; прогресс
                сохраняется, прерванный запуск продолжается автоматически.
              </p>
            </div>
          ) : (
            <div className="hint">…</div>
          )}
        </section>
      </div>

      <section className="card">
        <div className="card-head">
          <h2>{unmatchedOnly ? 'Чеки без операции в выписке' : 'Все чеки'}</h2>
          <label className="check">
            <input type="checkbox" checked={unmatchedOnly} onChange={(e) => setUnmatchedOnly(e.target.checked)} />
            только без операции
          </label>
        </div>
        <p className="hint">
          {int(total)} {plural(total, ['чек', 'чека', 'чеков'])}
          {unmatchedOnly ? ' без оплаты в выписке; такие чеки учтены как отдельные операции' : ''}
        </p>
        <table className="table">
          <thead>
            <tr>
              <th>Дата</th>
              <th>Продавец</th>
              <th className="num">Сумма</th>
              <th className="num">Позиций</th>
              <th>Операция</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {receipts.map((r) => {
              const active = open && open.id === r.id ? open.kind : null;
              return [
                <tr key={r.id}>
                  <td className="nowrap">{fmtDateTime(r.issued_at)}</td>
                  <td>
                    <div className="tx-title">{r.retail_place || shortSeller(r.seller_name) || '—'}</div>
                    {r.retail_place && <div className="tx-sub">{shortSeller(r.seller_name)}</div>}
                  </td>
                  <td className="num amount">{money(r.total)}</td>
                  <td className="num">{r.items_loaded ? int(r.item_count) : r.items_error ? '—' : '…'}</td>
                  <td>
                    {r.transaction_id && r.match_kind !== 'virtual' ? (
                      <Link className="badge badge-link" to={operationLink(r)} title="Открыть операцию">
                        {r.match_kind === 'manual' ? 'привязан вручную' : 'сопоставлен'} →
                      </Link>
                    ) : r.transaction_id && r.match_kind === 'virtual' ? (
                      <Link
                        className="badge badge-link badge-muted"
                        to={operationLink(r)}
                        title="В выписке нет такой оплаты, чек учтён как отдельная операция. Открыть её"
                      >
                        учтён отдельно →
                      </Link>
                    ) : r.items_loaded && r.cash_total + r.ecash_total === 0 ? (
                      <span className="hint" title="Зачёт предоплаты или покупка в кредит: деньги по этому чеку не двигались">
                        без движения денег
                      </span>
                    ) : (
                      <span className="hint">нет</span>
                    )}
                  </td>
                  <td className="num row-actions">
                    <button
                      type="button"
                      className={active === 'items' ? 'btn btn-small is-active' : 'btn btn-small'}
                      onClick={() => toggle(r, 'items')}
                    >
                      Состав
                    </button>
                    {r.transaction_id && r.match_kind !== 'virtual' ? (
                      <button type="button" className="btn btn-small" onClick={() => unlink(r)}>
                        Отвязать
                      </button>
                    ) : (
                      <button
                        type="button"
                        className={active === 'link' ? 'btn btn-small is-active' : 'btn btn-small'}
                        onClick={() => toggle(r, 'link')}
                      >
                        Найти операцию
                      </button>
                    )}
                  </td>
                </tr>,
                active && (
                  <tr key={`${r.id}-editor`} className="editor-row">
                    <td colSpan={6}>
                      {active === 'items' ? (
                        details[r.id] ? (
                          <ReceiptItems receipt={details[r.id]} />
                        ) : (
                          <div className="hint">Загружаем…</div>
                        )
                      ) : (
                        <CandidatePicker
                          receipt={r}
                          onError={fail}
                          onLinked={() => {
                            setOpen(null);
                            void loadReceipts();
                          }}
                        />
                      )}
                    </td>
                  </tr>
                ),
              ];
            })}
            {receipts.length === 0 && (
              <tr>
                <td colSpan={6} className="empty">
                  {session?.configured ? 'Чеков нет' : 'Подключите архив, чтобы загрузить чеки'}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </section>
    </div>
  );
}
