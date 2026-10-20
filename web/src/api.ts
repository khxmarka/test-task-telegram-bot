export type User = {
  id: number;
  display_name: string;
  username?: string;
  is_family_head: boolean;
};

export type Expense = {
  id: number;
  user_id: number;
  amount_byn_kopecks: number;
  category: string;
  spent_at: string;
  comment: string;
  user_display_name?: string;
};

export type CategorySummary = {
  category: string;
  total: number;
  count: number;
};

export type PeriodSummary = {
  total: number;
  count: number;
  from?: string;
  to?: string;
};

export type Filters = {
  user_id?: string;
  category?: string;
  from?: string;
  to?: string;
};

function authHeader(user: string, pass: string) {
  return "Basic " + btoa(`${user}:${pass}`);
}

async function apiGet<T>(path: string, user: string, pass: string, filters?: Filters): Promise<T> {
  const qs = new URLSearchParams();
  if (filters?.user_id) qs.set("user_id", filters.user_id);
  if (filters?.category) qs.set("category", filters.category);
  if (filters?.from) qs.set("from", filters.from);
  if (filters?.to) qs.set("to", filters.to);
  const q = qs.toString();
  const res = await fetch(`/api/v1${path}${q ? `?${q}` : ""}`, {
    headers: { Authorization: authHeader(user, pass) },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(body.message || `HTTP ${res.status}`);
  }
  return res.json();
}

export const fetchMembers = (u: string, p: string) =>
  apiGet<{ items: User[] }>("/members", u, p).then((r) => r.items);

export const fetchExpenses = (u: string, p: string, f: Filters) =>
  apiGet<{ items: Expense[] }>("/expenses", u, p, f).then((r) => r.items);

export const fetchSummary = (u: string, p: string, f: Filters) =>
  apiGet<PeriodSummary>("/expenses/summary", u, p, f);

export const fetchByCategory = (u: string, p: string, f: Filters) =>
  apiGet<{ items: CategorySummary[] }>("/expenses/by-category", u, p, f).then((r) => r.items);

export function formatBYN(kopecks: number) {
  return `${(kopecks / 100).toFixed(2)} BYN`;
}

export const CATEGORIES = [
  "stores",
  "cafes",
  "transport",
  "housing",
  "entertainment",
  "health",
  "shopping",
  "household",
  "other",
] as const;

export const CATEGORY_LABELS: Record<string, string> = {
  stores: "Магазины",
  cafes: "Кафе",
  transport: "Транспорт",
  housing: "Жильё",
  entertainment: "Развлечения",
  health: "Здоровье",
  shopping: "Покупки",
  household: "Бытовые",
  other: "Другое",
};
