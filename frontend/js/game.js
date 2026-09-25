// Game view: renders the three round types, submits guesses, shows
// the post-round reveal, and advances through the 10-round run.
// Rendered as a view inside / (set up by landing.js via showGame);
// state comes from sessionStorage, never the URL.
import { api } from "./api.js";

export const ROUNDS_PER_GAME = 10;

let mode = "daily";
let sessionId = "";
let roundIndex = 1;
let score = 0;

const hudRound = document.getElementById("hud-round");
const hudScore = document.getElementById("hud-score");
const root = document.getElementById("round-root");

// showGame swaps the landing view for the game view and starts round 1
// (or resumes at the stored round). State is (re)read here rather than
// at module load: landing.js imports this module before the session is
// created, and a second game in the same page load must pick up fresh
// state.
export function showGame() {
  const game = JSON.parse(sessionStorage.getItem("game") || "{}");
  mode = game.mode || "daily";
  sessionId = game.session || "";
  roundIndex = Number(game.round || 1);
  score = Number(game.score || 0);
  document.getElementById("landing-view").hidden = true;
  document.getElementById("end-view").hidden = true;
  document.getElementById("game-view").hidden = false;
  loadRound();
}

function saveState() {
  sessionStorage.setItem("game", JSON.stringify({
    mode, session: sessionId, round: roundIndex, score,
  }));
}

function renderHud() {
  hudRound.textContent = `Round ${roundIndex} of ${ROUNDS_PER_GAME}`;
  hudScore.textContent = `Score ${score}`;
}

async function loadRound() {
  renderHud();
  root.innerHTML = '<p class="landing__meta">Loading round…</p>';
  try {
    const round = mode === "daily"
      ? await api.daily.round(roundIndex)
      : await api.freeplay.round(sessionId, roundIndex);
    renderRound(round);
  } catch (err) {
    showError(err);
  }
}

function showError(err) {
  root.innerHTML = `<p class="landing__meta">${err.message}</p>
    <a class="btn btn--outline next-btn" href="/">Back to menu</a>`;
  console.error(err);
}

// ---- shared reveal + advance ----

function showResult({ correct, points, detail, onNext }) {
  const line = document.createElement("p");
  line.className = `result-line ${correct ? "good" : "bad"}`;
  line.textContent = correct ? `Correct! +${points}` : `Wrong. +${points}`;
  root.appendChild(line);
  if (detail) {
    const d = document.createElement("p");
    d.className = "landing__meta";
    d.textContent = detail;
    root.appendChild(d);
  }
  const next = document.createElement("button");
  next.className = "btn btn--primary next-btn";
  next.textContent = roundIndex >= ROUNDS_PER_GAME ? "Finish" : "Next round";
  next.addEventListener("click", () => {
    roundIndex += 1;
    if (roundIndex > ROUNDS_PER_GAME) {
      import("./end.js").then((m) => m.showEnd());
      return;
    }
    saveState();
    loadRound();
  });
  root.appendChild(next);
  onNext?.();
}

function submitAnswer(guess) {
  return mode === "daily"
    ? api.daily.answer(roundIndex, guess)
    : api.freeplay.answer(sessionId, roundIndex, guess);
}

function applyScore(points) {
  score += points;
  saveState();
  renderHud();
}

// ---- round type: higher / lower ----

function renderHigherLower(round) {
  root.innerHTML = `
    <p class="round-prompt">Which movie has the higher IMDb rating?</p>
    <div class="hl-board"></div>
    <div class="rating-badge" id="rating-badge"></div>`;
  const board = root.querySelector(".hl-board");
  let locked = false;

  round.movies.forEach((movie, i) => {
    const card = document.createElement("div");
    card.className = "hl-card";
    card.innerHTML = `
      <img src="${movie.poster_url}" alt="${movie.title} poster" />
      <h3>${movie.title}</h3>
      <p class="year">${movie.year}</p>`;
    card.addEventListener("click", async () => {
      if (locked) return;
      locked = true;
      board.querySelectorAll(".hl-card").forEach((c) => c.classList.add("locked"));
      try {
        const res = await submitAnswer({ choice: i });
        applyScore(res.points);
        revealHigherLower(round, res, i);
      } catch (err) {
        locked = false;
        board.querySelectorAll(".hl-card").forEach((c) => c.classList.remove("locked"));
        showError(err);
      }
    });
    board.appendChild(card);
  });
}

function revealHigherLower(round, res, chosen) {
  const cards = root.querySelectorAll(".hl-card");
  const badge = document.getElementById("rating-badge");
  const ratings = res.actual?.ratings || [round.movies.map(() => "?")[0], "?"];
  cards.forEach((card, i) => {
    if (i === chosen && res.correct) card.classList.add("correct");
    if (i === chosen && !res.correct) card.classList.add("wrong");
    if (i !== chosen && res.correct) card.classList.add("wrong");
  });
  badge.innerHTML = `${round.movies[0].title}: ${ratings[0]}<br>${round.movies[1].title}: ${ratings[1]}`;
  badge.classList.add("show");
  showResult({ correct: res.correct, points: res.points, detail: null });
}

// ---- round type: blurred poster ----

const BLUR_START = 40;
const BLUR_STEP = 8;

async function renderBlurred(round) {
  root.innerHTML = `
    <p class="round-prompt">Which movie is hiding behind the blur?</p>
    <img class="blur-img" id="blur-img" src="${round.poster_url}" alt="Mystery poster" style="--blur:${BLUR_START}px" />
    <div class="dots" id="dots"></div>
    <div class="search-wrap">
      <input type="text" id="guess-input" placeholder="Type a movie title…" autocomplete="off" />
      <ul class="ac-list" id="ac-list" hidden></ul>
    </div>`;
  const dots = document.getElementById("dots");
  for (let i = 0; i < 5; i++) {
    const d = document.createElement("span");
    d.className = "dot";
    dots.appendChild(d);
  }

  const input = document.getElementById("guess-input");
  const list = document.getElementById("ac-list");
  let titles = [];
  try {
    titles = (await api.pool.titles()).titles || [];
  } catch (err) {
    console.error("autocomplete titles unavailable", err);
  }
  let attempts = 0;
  let busy = false;

  function renderAc(query) {
    const matches = titles
      .filter((t) => t.toLowerCase().includes(query.toLowerCase()))
      .slice(0, 8);
    list.innerHTML = matches.map((t) => `<li>${t}</li>`).join("");
    list.hidden = matches.length === 0 || query.length === 0;
  }
  input.addEventListener("input", () => renderAc(input.value.trim()));
  list.addEventListener("click", (e) => {
    if (e.target.tagName === "LI") {
      input.value = e.target.textContent;
      list.hidden = true;
    }
  });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") submitGuess();
  });

  async function submitGuess() {
    const title = input.value.trim();
    if (!title || busy || attempts >= 5) return;
    busy = true;
    try {
      const res = await submitAnswer({ title });
      attempts = res.attempts ?? attempts + 1;
      if (res.correct) {
        finishBlurred(true, res);
        return;
      }
      markWrongAttempt(attempts);
    } catch (err) {
      showError(err);
      busy = false;
      return;
    }
    input.value = "";
    list.hidden = true;
    busy = false;
  }

  function markWrongAttempt(attemptNumber) {
    dots.children[attemptNumber - 1]?.classList.add("used");
    document.getElementById("blur-img").style.setProperty(
      "--blur", `${Math.max(BLUR_START - BLUR_STEP * attemptNumber, 0)}px`);
  }

  function finishBlurred(correct, res) {
    if (!correct) {
      dots.children[4]?.classList.add("used");
      document.getElementById("blur-img").style.setProperty("--blur", "0px");
    }
    input.disabled = true;
    applyScore(res.points);
    showResult({
      correct, points: res.points,
      detail: `The movie was “${res.actual?.title ?? "?"}”`,
    });
  }

  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") submitGuess();
  });
  list.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && e.target.tagName === "LI") {
      input.value = e.target.textContent;
      list.hidden = true;
      submitGuess();
    }
  });
}

// ---- round type: guess the year ----

function renderGuessYear(round) {
  const movie = round.movies[0];
  const min = round.year_min ?? 1920;
  const max = round.year_max ?? 2026;
  root.innerHTML = `
    <p class="round-prompt">When was this released?</p>
    <img src="${movie.poster_url}" alt="${movie.title} poster" style="width:min(60vw,260px);border-radius:12px" />
    <h3>${movie.title}</h3>
    <p class="year-label" id="year-label">${min}</p>
    <input type="range" class="year-slider" id="year-slider" min="${min}" max="${max}" value="${min}" />
    <div class="year-bounds"><span>${min}</span><span>${max}</span></div>
    <button class="btn btn--primary next-btn" id="lock-year">Lock in</button>`;

  const slider = document.getElementById("year-slider");
  const label = document.getElementById("year-label");
  slider.addEventListener("input", () => { label.textContent = slider.value; });

  document.getElementById("lock-year").addEventListener("click", async (e) => {
    e.target.disabled = true;
    slider.disabled = true;
    try {
      const res = await submitAnswer({ year: Number(slider.value) });
      applyScore(res.points);
      const detail = res.actual?.year != null
        ? `It was ${res.actual.year} (you were off by ${Math.abs(res.actual.year - Number(slider.value))}).`
        : null;
      showResult({ correct: res.correct, points: res.points, detail });
    } catch (err) {
      e.target.disabled = false;
      slider.disabled = false;
      showError(err);
    }
  });
}

// ---- dispatch ----

export function renderRound(round) {
  document.body.dataset.roundType = round.type;
  switch (round.type) {
    case "higher_lower": return renderHigherLower(round);
    case "blurred_poster": return renderBlurred(round);
    case "guess_the_year": return renderGuessYear(round);
    default: return showError(new Error(`Unknown round type: ${round.type}`));
  }
}

loadRound();
