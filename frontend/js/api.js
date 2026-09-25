import { API_BASE } from "./config.js";

export class APIError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new APIError(body.error || `Request failed: ${res.status}`, res.status);
  }
  return res.json();
}

export const api = {
  daily: {
    status: () => request("/daily/status"),
    start: () => request("/daily/start", { method: "POST" }),
    round: (n) => request(`/daily/round/${n}`),
    answer: (n, guess) =>
      request(`/daily/round/${n}/answer`, { method: "POST", body: JSON.stringify({ guess }) }),
    result: () => request("/daily/result"),
    submitLeaderboard: (name) =>
      request("/daily/leaderboard", { method: "POST", body: JSON.stringify({ name }) }),
  },
  freeplay: {
    start: () => request("/freeplay/start", { method: "POST" }),
    round: (id, n) => request(`/freeplay/${id}/round/${n}`),
    answer: (id, n, guess) =>
      request(`/freeplay/${id}/round/${n}/answer`, { method: "POST", body: JSON.stringify({ guess }) }),
    result: (id) => request(`/freeplay/${id}/result`),
  },
  pool: {
    titles: () => request("/pool/titles"),
  },
};
