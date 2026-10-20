import { FormEvent, useEffect, useMemo, useState } from "react";
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
  Filters,
  PeriodSummary,
  User,
  fetchByCategory,
  fetchExpenses,
  fetchMembers,
  fetchSummary,
  formatBYN,
} from "./api";

const COLORS = ["#e2b15a", "#7ec8a3", "#8eb6e8", "#e07a6a", "#c4a0e8", "#9ad0c2", "#f0c27a", "#a8b7c7", "#d7c4a4"];

type Creds = { user: string; pass: string };

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
  }, [creds, filters]);

  const chartData = useMemo(
    () =>
      byCat.map((c) => ({
        name: CATEGORY_LABELS[c.category] || c.category,
        value: c.total / 100,
      })),
    [byCat]
  );

  if (!creds) {
    return (
      <div className="login">
        <form className="login form" onSubmit={onLogin}>
          <h1>Семейные расходы</h1>
          <p>Панель главы семьи. Только просмотр.</p>
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
    <div className="shell">
      <header className="hero">
        <div>
          <h1>Семейные расходы</h1>
          <p>Аналитика трат всей семьи. Мутации только через Telegram-бота.</p>
        </div>
        <button type="button" onClick={logout}>
          Выйти
        </button>
      </header>

      <section className="filters">
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
        <button type="button" onClick={() => setFilters({})}>
          Сбросить
        </button>
      </section>

      {error && <p className="error">{error}</p>}

      <section className="stats">
        <div className="panel">
          <h2>Сводка</h2>
          <p className="big">{summary ? formatBYN(summary.total) : "—"}</p>
          <p className="meta">{summary ? `${summary.count} записей` : ""}</p>
        </div>
        <div className="panel">
          <h2>По категориям</h2>
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
      </section>

      <section className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Дата</th>
              <th>Кто</th>
              <th>Сумма</th>
              <th>Категория</th>
              <th>Комментарий</th>
            </tr>
          </thead>
          <tbody>
            {expenses.map((e) => (
              <tr key={e.id}>
                <td>{e.spent_at.slice(0, 10)}</td>
                <td>{e.user_display_name || e.user_id}</td>
                <td>{formatBYN(e.amount_byn_kopecks)}</td>
                <td>{CATEGORY_LABELS[e.category] || e.category}</td>
                <td>{e.comment || "—"}</td>
              </tr>
            ))}
            {expenses.length === 0 && (
              <tr>
                <td colSpan={5}>Трат пока нет</td>
              </tr>
            )}
          </tbody>
        </table>
      </section>
    </div>
  );
}
