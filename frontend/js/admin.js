// Admin reveal page: password-gated view of today's full round set with
// correct answers. Sends X-Admin-Password; the backend rate-limits the
// IP after 5 failed attempts for 15 minutes.
import { API_BASE } from "./config.js";

const form = document.getElementById("reveal-form");
const passwordInput = document.getElementById("admin-password");
const msg = document.getElementById("admin-msg");
const roundsRoot = document.getElementById("rounds-root");

const TYPE_LABELS = {
  higher_lower: "Higher or Lower",
  blurred_poster: "Blurred Poster",
  guess_the_year: "Guess the Year",
};

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  msg.textContent = "";
  roundsRoot.innerHTML = "";
  const password = passwordInput.value;
  if (!password) return;

  try {
    const res = await fetch(`${API_BASE}/admin/daily/reveal`, {
      headers: { "X-Admin-Password": password },
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `Request failed: ${res.status}`);
    }
    renderRounds(await res.json());
    passwordInput.value = "";
  } catch (err) {
    msg.textContent = err.message;
  }
});

function renderRounds(rounds) {
  if (!Array.isArray(rounds) || rounds.length === 0) {
    msg.textContent = "No game generated for today yet.";
    return;
  }
  const grid = document.createElement("div");
  grid.className = "rounds-grid";

  rounds.forEach((round) => {
    const card = document.createElement("article");
    card.className = "admin-round";
    card.innerHTML = `<h3>Round ${round.index} — ${TYPE_LABELS[round.type] || round.type}</h3>`;
    card.appendChild(answerLine(round));
    (round.movies || []).forEach((m) => {
      const img = document.createElement("img");
      img.src = round.type === "blurred_poster"
        ? (m.textless_poster_url || m.poster_url)
        : m.poster_url;
      img.alt = m.title;
      img.loading = "lazy";
      card.appendChild(img);
      const cap = document.createElement("p");
      cap.innerHTML = `${m.title} (${m.year}) — <span class="answer">★ ${m.imdb_rating}</span>`;
      card.appendChild(cap);
    });
    grid.appendChild(card);
  });

  roundsRoot.appendChild(grid);
}

function answerLine(round) {
  const p = document.createElement("p");
  switch (round.type) {
    case "higher_lower": {
      const best = (round.movies || []).reduce((a, b) => (b.imdb_rating > a.imdb_rating ? b : a));
      p.innerHTML = `Answer: <span class="answer">${best.title}</span> (★ ${best.imdb_rating})`;
      break;
    }
    case "guess_the_year":
      p.innerHTML = `Answer: <span class="answer">${round.movies?.[0]?.year ?? "?"}</span>`;
      break;
    case "blurred_poster":
      p.innerHTML = `Answer: <span class="answer">${round.movies?.[0]?.title ?? "?"}</span>`;
      break;
  }
  return p;
}
