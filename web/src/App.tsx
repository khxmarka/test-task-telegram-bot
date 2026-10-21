import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
} from "recharts";
import {
  CATEGORIES,
  CATEGORY_LABELS,
  CategorySummary,
  Expense,
  ExpenseWrite,
  Filters,
  PeriodSummary,
  User,
  createExpense,
  deleteExpense,
  fetchByCategory,
  fetchExpenses,
  fetchMembers,
  fetchSummary,
  formatBYN,
  updateExpense,
} from "./api";

const COLORS = [
  "#2271b1",
  "#00a32a",
  "#dba617",
  "#d63638",
  "#9b51e0",
  "#00ba37",
  "#72aee6",
  "#646970",
  "#b32d2e",
];

type Creds = { user: string; pass: string };

type FormState = {
  user_id: string;
  amount: string;
  category: string;
  spent_at: string;
  comment: string;
};

const emptyForm = (): FormState => ({
  user_id: "",
  amount: "",
  category: "other",
  spent_at: new Date().toISOString().slice(0, 10),
  comment: "",
});

export default function App() {
  const [creds, setCreds] = useState<Creds | null>(() => {
    const raw = sessionStorage.getItem("family_auth");
    return raw ? (JSON.parse(raw) as Creds) : null;
  });
  const [loginUser, setLoginUser] = useState("admin");
  const [loginPass, setLoginPass] = useState("admin");
  const [loginError, setLoginError] = useState("");

  const [members, setMembers] = useState<User[]>([]);
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [summary, setSummary] = useState<PeriodSummary | null>(null);
  const [byCat, setByCat] = useState<CategorySummary[]>([]);
  const [error, setError] = useState("");
  const [filters, setFilters] = useState<Filters>({});
  const [reloadKey, setReloadKey] = useState(0);

  const [modal, setModal] = useState<"create" | "edit" | null>(null);
  const [editing, setEditing] = useState<Expense | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [saving, setSaving] = useState(false);

  async function onLogin(e: FormEvent) {
    e.preventDefault();
    setLoginError("");
    try {
      await fetchMembers(loginUser, loginPass);
      const c = { user: loginUser, pass: loginPass };
      sessionStorage.setItem("family_auth", JSON.stringify(c));
      setCreds(c);
    } catch (err) {
      setLoginError(err instanceof Error ? err.message : "Ошибка входа");
    }
  }

  function logout() {
    sessionStorage.removeItem("family_auth");
    setCreds(null);
  }

  const refresh = useCallback(() => setReloadKey((k) => k + 1), []);

  useEffect(() => {
    if (!creds) return;
    let cancelled = false;
    (async () => {
      setError("");
      try {
        const [m, ex, sum, cats] = await Promise.all([
          fetchMembers(creds.user, creds.pass),
          fetchExpenses(creds.user, creds.pass, filters),
          fetchSummary(creds.user, creds.pass, filters),
          fetchByCategory(creds.user, creds.pass, filters),
        ]);
        if (cancelled) return;
        setMembers(m);
        setExpenses(ex);
        setSummary(sum);
        setByCat(cats);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Ошибка загрузки");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [creds, filters, reloadKey]);

  const chartData = useMemo(
    () =>
      byCat.map((c) => ({
        name: CATEGORY_LABELS[c.category] || c.category,
        value: c.total / 100,
      })),
    [byCat]
  );

  function openCreate() {
    const f = emptyForm();
    if (members[0]) f.user_id = String(members[0].id);
    setForm(f);
    setEditing(null);
    setModal("create");
  }

  function openEdit(e: Expense) {
    setEditing(e);
    setForm({
      user_id: String(e.user_id),
      amount: (e.amount_byn_kopecks / 100).toFixed(2),
      category: e.category,
      spent_at: e.spent_at.slice(0, 10),
      comment: e.comment || "",
    });
    setModal("edit");
  }

  async function onSave(ev: FormEvent) {
    ev.preventDefault();
    if (!creds) return;
    const amount = parseFloat(form.amount.replace(",", "."));
    if (!form.user_id || !(amount > 0)) {
      setError("Укажите члена семьи и сумму > 0");
      return;
    }
    const payload: ExpenseWrite = {
      user_id: Number(form.user_id),
      amount,
      currency: "BYN",
      category: form.category,
      spent_at: form.spent_at,
      comment: form.comment,
    };
    setSaving(true);
    setError("");
    try {
      if (modal === "create") {
        await createExpense(creds.user, creds.pass, payload);
      } else if (modal === "edit" && editing) {
        await updateExpense(creds.user, creds.pass, editing.id, payload);
      }
      setModal(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка сохранения");
    } finally {
      setSaving(false);
    }
  }

  async function onDelete(e: Expense) {
    if (!creds) return;
    if (!window.confirm(`Удалить трату #${e.id}?`)) return;
    setError("");
    try {
      await deleteExpense(creds.user, creds.pass, e.id);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка удаления");
    }
  }

  if (!creds) {
    return (
      <div className="login">
        <form className="loginform" onSubmit={onLogin}>
          <h1>Семейные расходы</h1>
          <p>Вход в консоль главы семьи</p>
          <label>
            Логин
            <input value={loginUser} onChange={(e) => setLoginUser(e.target.value)} autoComplete="username" />
          </label>
          <label>
            Пароль
            <input
              type="password"
              value={loginPass}
              onChange={(e) => setLoginPass(e.target.value)}
              autoComplete="current-password"
            />
          </label>
          {loginError && <p className="error">{loginError}</p>}
          <button type="submit">Войти</button>
        </form>
      </div>
    );
  }

  return (
    <div className="wp">
      <header className="admin-bar">
        <strong>Семейные расходы</strong>
        <button type="button" onClick={logout}>
          Выйти
        </button>
      </header>

      <div className="wp-body">
        <nav className="sidebar">
          <button type="button" className="menu-item active">
            Консоль
          </button>
          <button type="button" className="menu-item active">
            Траты
          </button>
        </nav>

        <main className="content">
          <h1>Консоль</h1>

          {error && <p className="error">{error}</p>}

          <div className="widgets">
            <div className="widget">
              <h2 className="widget-header">Сводка</h2>
              <div className="widget-body">
                <p className="big">{summary ? formatBYN(summary.total) : "—"}</p>
                <p className="meta">{summary ? `${summary.count} записей` : ""}</p>
              </div>
            </div>
            <div className="widget">
              <h2 className="widget-header">По категориям</h2>
              <div className="widget-body">
                <div style={{ width: "100%", height: 220 }}>
                  {chartData.length === 0 ? (
                    <p className="meta">Нет данных</p>
                  ) : (
                    <ResponsiveContainer>
                      <PieChart>
                        <Pie data={chartData} dataKey="value" nameKey="name" outerRadius={80}>
                          {chartData.map((_, i) => (
                            <Cell key={i} fill={COLORS[i % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip formatter={(v: number) => `${v.toFixed(2)} BYN`} />
                        <Legend />
                      </PieChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </div>
            </div>
          </div>

          <div className="widget">
            <h2 className="widget-header">Траты семьи</h2>
            <div className="widget-body">
              <div className="toolbar">
                <div className="filters-row">
                  <label>
                    Член семьи
                    <select
                      value={filters.user_id || ""}
                      onChange={(e) => setFilters((f) => ({ ...f, user_id: e.target.value || undefined }))}
                    >
                      <option value="">Все</option>
                      {members.map((m) => (
                        <option key={m.id} value={String(m.id)}>
                          {m.display_name}
                          {m.is_family_head ? " (глава)" : ""}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Категория
                    <select
                      value={filters.category || ""}
                      onChange={(e) => setFilters((f) => ({ ...f, category: e.target.value || undefined }))}
                    >
                      <option value="">Все</option>
                      {CATEGORIES.map((c) => (
                        <option key={c} value={c}>
                          {CATEGORY_LABELS[c]}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    С
                    <input
                      type="date"
                      value={filters.from || ""}
                      onChange={(e) => setFilters((f) => ({ ...f, from: e.target.value || undefined }))}
                    />
                  </label>
                  <label>
                    По
                    <input
                      type="date"
                      value={filters.to || ""}
                      onChange={(e) => setFilters((f) => ({ ...f, to: e.target.value || undefined }))}
                    />
                  </label>
                  <button type="button" className="btn btn-secondary" onClick={() => setFilters({})}>
                    Сбросить
                  </button>
                </div>
                <button type="button" className="btn" onClick={openCreate}>
                  Добавить трату
                </button>
              </div>

              <div className="table-wrap">
                <table className="wp-list">
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>Дата</th>
                      <th>Кто</th>
                      <th>Сумма</th>
                      <th>Категория</th>
                      <th>Комментарий</th>
                      <th>Действия</th>
                    </tr>
                  </thead>
                  <tbody>
                    {expenses.map((e) => (
                      <tr key={e.id}>
                        <td>{e.id}</td>
                        <td>{e.spent_at.slice(0, 10)}</td>
                        <td>{e.user_display_name || e.user_id}</td>
                        <td>{formatBYN(e.amount_byn_kopecks)}</td>
                        <td>{CATEGORY_LABELS[e.category] || e.category}</td>
                        <td>{e.comment || "—"}</td>
                        <td>
                          <div className="row-actions">
                            <button type="button" className="btn btn-secondary" onClick={() => openEdit(e)}>
                              Изменить
                            </button>
                            <button type="button" className="btn btn-danger" onClick={() => onDelete(e)}>
                              Удалить
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                    {expenses.length === 0 && (
                      <tr>
                        <td colSpan={7}>Трат пока нет</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </main>
      </div>

      {modal && (
        <div className="modal-backdrop" onClick={() => !saving && setModal(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>{modal === "create" ? "Добавить трату" : `Изменить #${editing?.id}`}</h2>
            <form className="form-grid" onSubmit={onSave}>
              <label>
                Член семьи
                <select
                  value={form.user_id}
                  onChange={(e) => setForm((f) => ({ ...f, user_id: e.target.value }))}
                  disabled={modal === "edit"}
                  required
                >
                  <option value="" disabled>
                    Выберите
                  </option>
                  {members.map((m) => (
                    <option key={m.id} value={String(m.id)}>
                      {m.display_name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Сумма (BYN)
                <input
                  type="number"
                  step="0.01"
                  min="0.01"
                  value={form.amount}
                  onChange={(e) => setForm((f) => ({ ...f, amount: e.target.value }))}
                  required
                />
              </label>
              <label>
                Категория
                <select
                  value={form.category}
                  onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))}
                >
                  {CATEGORIES.map((c) => (
                    <option key={c} value={c}>
                      {CATEGORY_LABELS[c]}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Дата
                <input
                  type="date"
                  value={form.spent_at}
                  onChange={(e) => setForm((f) => ({ ...f, spent_at: e.target.value }))}
                  required
                />
              </label>
              <label>
                Комментарий
                <input
                  value={form.comment}
                  onChange={(e) => setForm((f) => ({ ...f, comment: e.target.value }))}
                />
              </label>
              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" disabled={saving} onClick={() => setModal(null)}>
                  Отмена
                </button>
                <button type="submit" className="btn" disabled={saving}>
                  {saving ? "Сохранение…" : "Сохранить"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
