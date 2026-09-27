// Game view: renders the three round types, submits guesses, shows
// the post-round reveal, and advances through the 10-round run.
// Rendered as a view inside / (set up by landing.js via showGame);
// state comes from localStorage, never the URL. Refreshing mid-game
// resumes at the stored round; a finished game never auto-resumes.
import { api } from "./api.js";

export const ROUNDS_PER_GAME = 10;

let mode = "daily";
let sessionId = "";
let roundIndex = 1;
let score = 0;

// One storage key per mode (game:freeplay / game:daily) so starting the
// daily run never clobbers a saved Free Play session, plus a game:active
// marker recording which mode is in play and whether the player left it
// deliberately (the ✕ button) — that flag is what lets a mid-game
// refresh resume while a post-exit refresh stays on the landing page.
const stateKey = (m) => `game:${m}`;
const activeKey = "game:active";

function readState(m) {
  return JSON.parse(localStorage.getItem(stateKey(m)) || "null");
}

function setActiveMode(m, exited) {
  localStorage.setItem(activeKey, JSON.stringify({ mode: m, exited }));
}

function activeMode() {
  const a = JSON.parse(localStorage.getItem(activeKey) || "null");
  return a ? a.mode : null;
}

const hudRound = document.getElementById("hud-round");
const hudScore = document.getElementById("hud-score");
const root = document.getElementById("round-root");

// showGame swaps the landing view for the game view and starts round 1
// (or resumes at the stored round for the given mode). State is (re)read
// here rather than at module load: landing.js imports this module before
// a session is created, and a second game in the same page load must
// pick up fresh state.
export function showGame(m) {
  mode = m || "daily";
  const game = readState(mode) || {};
  sessionId = game.session || "";
  roundIndex = Number(game.round || 1);
  score = Number(game.score || 0);
  setActiveMode(mode, false);
  document.getElementById("landing-view").hidden = true;
  document.getElementById("end-view").hidden = true;
  document.getElementById("game-view").hidden = false;
  loadRound();
}

function saveState(finished = false) {
  localStorage.setItem(stateKey(mode), JSON.stringify({
    mode, session: sessionId, round: roundIndex, score, finished,
  }));
}

function renderHud() {
  // Display the round being PLAYED (roundIndex is the server's next
  // round after an answer; the label catches up when it loads).
  const shown = roundIndex > ROUNDS_PER_GAME ? ROUNDS_PER_GAME : roundIndex;
  hudRound.textContent = `Round ${shown} of ${ROUNDS_PER_GAME}`;
  hudScore.textContent = `Score ${score}`;
}

// Menu (✕): back to the landing view without destroying the session —
// a run can still be resumed via its Play/Free Play button. The exited
// flag makes a subsequent refresh stay on the landing page instead of
// jumping back into the game.
export function showMenu() {
  if (mode) setActiveMode(mode, true);
  document.getElementById("game-view").hidden = true;
  document.getElementById("end-view").hidden = true;
  document.getElementById("landing-view").hidden = false;
  window.dispatchEvent(new CustomEvent("game:exit"));
}
window.gameShowMenu = showMenu;

async function loadRound() {
  hudRound.textContent = `Round ${Math.min(roundIndex, ROUNDS_PER_GAME)} of ${ROUNDS_PER_GAME}`;
  hudScore.textContent = `Score ${score}`;
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
  const dead = /completed|not found/i.test(err.message || "");
  if (dead) {
    // The session can no longer be played: clear it so the next page
    // load starts fresh at the menu instead of retrying a dead session.
    localStorage.removeItem("game");
  }
  root.innerHTML = `<p class="landing__meta">${err.message}</p>`;
  if (!dead) {
    // Transient failure (network, pool empty): the session is fine —
    // offer a retry at the same round.
    const retry = document.createElement("button");
    retry.className = "btn btn--primary next-btn";
    retry.textContent = "Retry";
    retry.addEventListener("click", loadRound);
    root.appendChild(retry);
  }
  const menu = document.createElement("a");
  menu.className = "btn btn--outline next-btn";
  menu.href = "/";
  menu.textContent = "Back to menu";
  root.appendChild(menu);
  console.error(err);
}

// ---- shared reveal + advance ----

// Result headline: a wrong answer can still earn points (year rounds,
// later blurred attempts), so "Wrong. +8" would read as a contradiction.
function resultLabel(correct, points) {
  if (correct) return `Correct! +${points}`;
  if (points > 0) return `So close! +${points}`;
  return `Wrong. +${points}`;
}

function showResult({ correct, points, detail, onNext }) {
  const line = document.createElement("p");
  line.className = `result-line ${correct ? "good" : points > 0 ? "mid" : "bad"}`;
  line.textContent = resultLabel(correct, points);
  root.appendChild(line);
  if (detail) {
    const d = document.createElement("div");
    d.className = "answer-card";
    d.textContent = detail;
    root.appendChild(d);
  }
  const next = document.createElement("button");
  next.className = "btn btn--primary next-btn";
  next.textContent = roundIndex > ROUNDS_PER_GAME ? "Finish" : "Next round";
  next.addEventListener("click", () => {
    if (roundIndex > ROUNDS_PER_GAME) {
      saveState(true); // finished: a refresh must not resume this session
      import("./end.js").then((m) => m.showEnd());
      return;
    }
    saveState();
    loadRound();
  });
  root.appendChild(next);
  // Bring the result into view (the reveal content above can be tall).
  line.scrollIntoView({ behavior: "smooth", block: "center" });
  onNext?.();
}

// syncProgress adopts the server's authoritative position after every
// answer: the saved round is the server's, so refreshing at any moment
// (mid-round, result screen, after finish) resumes correctly instead of
// resuming at a stale round and 409ing on the next submit.
function syncProgress(res) {
  score += res.points;
  roundIndex = res.next_round;
  saveState(res.finished);
  hudScore.textContent = `Score ${score}`; // round label updates on loadRound
}

function submitAnswer(guess) {
  return mode === "daily"
    ? api.daily.answer(roundIndex, guess)
    : api.freeplay.answer(sessionId, roundIndex, guess);
}

// ---- round type: higher / lower ----

function renderHigherLower(round) {
  root.innerHTML = `
    <p class="round-prompt">Which movie has the higher rating?</p>
    <div class="hl-board"></div>`;
  const board = root.querySelector(".hl-board");
  let locked = false;

  round.movies.forEach((movie, i) => {
    const card = document.createElement("div");
    card.className = "hl-card";
    card.innerHTML = `
      <div class="poster-wrap">
        <img src="${movie.poster_url}" alt="${movie.title} poster" />
        <span class="rating-pill" id="pill-${i}" hidden></span>
      </div>
      <h3>${movie.title}</h3>
      <p class="year">${movie.year}</p>`;
    card.addEventListener("click", async () => {
      if (locked) return;
      locked = true;
      board.querySelectorAll(".hl-card").forEach((c) => c.classList.add("locked"));
      try {
        const res = await submitAnswer({ choice: i });
        syncProgress(res);
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
  const ratings = res.actual?.ratings || ["?", "?"];
  // Ratings appear on each card (not an overlay covering the posters);
  // the winner's pill is gold, the loser's dimmed.
  cards.forEach((card, i) => {
    const pill = card.querySelector(".rating-pill");
    pill.textContent = `★ ${ratings[i]}`;
    pill.hidden = false;
    const isWinner = i === 0 ? ratings[0] > ratings[1] : ratings[1] > ratings[0];
    if (isWinner) pill.classList.add("win");
    // The winner's card is emerald, the loser's rose — regardless of
    // which card the player tapped.
    card.classList.add(i === chosen ? (res.correct ? "correct" : "wrong")
      : (res.correct ? "wrong" : "correct"));
  });
  showResult({ correct: res.correct, points: res.points, detail: null });
}

// ---- round type: blurred poster ----

const BLUR_START = 30;
const BLUR_STEP = 7;

async function renderBlurred(round) {
  root.innerHTML = `
    <p class="round-prompt">Guess the movie!</p>
    <img class="blur-img" id="blur-img" src="${round.poster_url}" alt="Mystery poster" style="--blur:${BLUR_START}px" />
    <div class="dots" id="dots"></div>
    <div class="search-wrap">
      <input type="text" id="guess-input" placeholder="Type a movie title…" autocomplete="off" />
      <ul class="ac-list" id="ac-list" hidden></ul>
    </div>
    <div class="blur-actions">
      <button class="btn btn--primary" id="blur-submit">Submit</button>
      <button class="btn btn--outline" id="blur-skip">Skip</button>
    </div>`;
  const dots = document.getElementById("dots");
  for (let i = 0; i < 5; i++) {
    const d = document.createElement("span");
    d.className = "dot";
    dots.appendChild(d);
  }

  const input = document.getElementById("guess-input");
  const list = document.getElementById("ac-list");
  const submitBtn = document.getElementById("blur-submit");
  const skipBtn = document.getElementById("blur-skip");
  // Restore mid-round state after a reload: the server's attempt count
  // drives the dots, blur level, and Skip visibility.
  let attempts = Math.min(round.attempts ?? 0, 5);
  for (let i = 1; i <= attempts; i++) markWrongAttempt(i);
  let titles = [];
  try {
    titles = (await api.pool.titles()).titles || [];
  } catch (err) {
    console.error("autocomplete titles unavailable", err);
  }
  const titleOf = (t) => (typeof t === "string" ? t : t.title);
  let busy = false;
  const updateSkipVisibility = () => { skipBtn.hidden = attempts >= 4; };
  updateSkipVisibility();

  function renderAc(query) {
    const matches = titles
      .filter((t) => titleOf(t).toLowerCase().includes(query.toLowerCase()))
      .sort((a, b) => titleOf(a).localeCompare(titleOf(b)))
      .slice(0, 8);
    // "Title (Year)" labels; selecting fills the input with the title
    // only — the submitted guess remains the title.
    list.innerHTML = matches
      .map((t) => `<li data-title="${titleOf(t).replace(/"/g, "&quot;")}">${titleOf(t)} (${typeof t === "string" ? "" : t.year})</li>`)
      .join("");
    list.hidden = matches.length === 0 || query.length === 0;
  }
  input.addEventListener("input", () => renderAc(input.value.trim()));
  list.addEventListener("click", (e) => {
    if (e.target.tagName === "LI") {
      input.value = e.target.dataset.title;
      list.hidden = true;
      input.focus();
    }
  });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") submitGuess();
  });
  submitBtn.addEventListener("click", submitGuess);
  // Skip = give up on this attempt without a guess. It scores 0 and
  // burns an attempt exactly like a wrong answer (empty title), and
  // disappears on the last attempt where only a guess can score.
  skipBtn.addEventListener("click", async () => {
    if (busy || attempts >= 4) return;
    busy = true;
    submitBtn.disabled = true;
    skipBtn.disabled = true;
    try {
      const res = await submitAnswer({ title: "" });
      attempts = res.attempts ?? attempts + 1;
      syncProgress(res);
      markWrongAttempt(attempts);
      updateSkipVisibility();
      if (attempts >= 5) {
        finishBlurred(false, res);
        return;
      }
    } catch (err) {
      showError(err);
      busy = false;
      skipBtn.disabled = false;
      submitBtn.disabled = false;
      return;
    }
    busy = false;
    skipBtn.disabled = false;
    submitBtn.disabled = false;
  });

  async function submitGuess() {
    const title = input.value.trim();
    if (!title || busy || attempts >= 5) return;
    busy = true;
    submitBtn.disabled = true;
    skipBtn.disabled = true;
    try {
      const res = await submitAnswer({ title });
      attempts = res.attempts ?? attempts + 1;
      syncProgress(res); // server-authoritative round/score, even mid-round
      if (res.correct) {
        finishBlurred(true, res);
        return;
      }
      markWrongAttempt(attempts);
      updateSkipVisibility();
      if (attempts >= 5) {
        // 5th wrong attempt: the round is over — reveal and move on.
        finishBlurred(false, res);
        return;
      }
    } catch (err) {
      showError(err);
      busy = false;
      return;
    }
    input.value = "";
    list.hidden = true;
    busy = false;
    submitBtn.disabled = false;
    skipBtn.disabled = false;
    updateSkipVisibility();
  }

  function markWrongAttempt(attemptNumber) {
    dots.children[attemptNumber - 1]?.classList.add("used");
    document.getElementById("blur-img").style.setProperty(
      "--blur", `${Math.max(BLUR_START - BLUR_STEP * attemptNumber, 0)}px`);
  }

  function finishBlurred(correct, res) {
    // Round over — reveal the poster whether the guess was right or wrong.
    if (!correct) {
      dots.children[4]?.classList.add("used");
    }
    document.getElementById("blur-img").style.setProperty("--blur", "0px");
    input.disabled = true;
    list.hidden = true;
    submitBtn.hidden = true;
    skipBtn.hidden = true;
    showResult({
      correct, points: res.points,
      detail: `The movie was “${res.actual?.title ?? "?"}”`,
    });
  }

  list.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && e.target.tagName === "LI") {
      input.value = e.target.dataset.title;
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
    <button class="btn btn--primary next-btn" id="year-submit">Submit</button>`;

  const slider = document.getElementById("year-slider");
  const label = document.getElementById("year-label");
  const btn = document.getElementById("year-submit");
  let answered = false;
  slider.addEventListener("input", () => { label.textContent = slider.value; });

  // One button, two phases: Submit locks the guess in and shows the
  // result; it then becomes Next round (Finish on the last round).
  btn.addEventListener("click", async () => {
    if (answered) {
      // Second click: advance. roundIndex was already synced to the
      // server's next round by syncProgress — do NOT increment again
      // (that skipped a round and desynced client from server).
      if (roundIndex > ROUNDS_PER_GAME) {
        saveState(true);
        import("./end.js").then((m) => m.showEnd());
        return;
      }
      saveState();
      loadRound();
      return;
    }
    answered = true;
    btn.disabled = true;
    slider.disabled = true;
    try {
      const res = await submitAnswer({ year: Number(slider.value) });
      syncProgress(res);
      const off = Math.abs(res.actual.year - Number(slider.value));
      const detail = res.actual?.year != null
        ? `“${movie.title}” was released in ${res.actual.year} — you were off by ${off} year${off === 1 ? "" : "s"}.`
        : null;
      const line = document.createElement("p");
      line.className = `result-line ${res.correct ? "good" : res.points > 0 ? "mid" : "bad"}`;
      line.textContent = resultLabel(res.correct, res.points);
      root.appendChild(line);
      if (detail) {
        const d = document.createElement("div");
        d.className = "answer-card";
        d.textContent = detail;
        root.appendChild(d);
      }
      btn.textContent = roundIndex >= ROUNDS_PER_GAME ? "Finish" : "Next round";
      btn.disabled = false;
    } catch (err) {
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

// Resume on refresh — only when the player is mid-game on an
// unfinished, non-exited session: it continues at the stored round.
// A finished session does NOT reopen the end view — the landing page's
// loadStatus (server-driven) offers the Leaderboard button instead, and
// after midnight it correctly offers a fresh Game of the Day.
(() => {
  const m = activeMode();
  if (!m) return;
  const saved = readState(m);
  const a = JSON.parse(localStorage.getItem(activeKey) || "{}");
  if (saved && saved.session && !saved.finished && !a.exited) {
    showGame(m);
  }
})();
