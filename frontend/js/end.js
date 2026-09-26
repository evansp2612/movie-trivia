// End view: final score, daily leaderboard submission + top-10 (Daily
// only), and return-to-main-menu. Rendered as a view inside "/" — there
// is no /end URL and no dedicated replay control, per the PRD.
import { api } from "./api.js";

// The mode whose run just finished (recorded by game.js in game:active).
function activeMode() {
  const a = JSON.parse(localStorage.getItem("game:active") || "null");
  return a ? a.mode : "daily";
}
function readState(m) {
  return JSON.parse(localStorage.getItem(`game:${m}`) || "null");
}
function writeState(m, state) {
  localStorage.setItem(`game:${m}`, JSON.stringify(state));
}
function setActiveMode(m, exited) {
  localStorage.setItem("game:active", JSON.stringify({ mode: m, exited }));
}

export function renderEndScreen({ score, breakdown }) {
  const scoreEl = document.getElementById("final-score");
  const detail = document.getElementById("end-detail");
  scoreEl.textContent = `${score}`;

  if (breakdown) {
    detail.textContent = Object.entries(breakdown)
      .map(([type, pts]) => `${type}: ${pts}`)
      .join(" · ");
  }
}

// Daily only (PRD: Free Play has no leaderboard): name input + one-shot
// submission, then the ordered top-10 for today. serverScore is the
// finished run's score from the result endpoint. alreadySubmitted comes
// from the backend status — authoritative across tabs and devices. The
// submitted name is kept locally only to highlight the player's row.
async function renderLeaderboard(root, serverScore, alreadySubmitted) {
  const state = readState("daily") || {};
  let submittedName = state.submittedName || null;

  const section = document.createElement("div");
  section.className = "leaderboard";

  const input = document.createElement("input");
  input.type = "text";
  input.maxLength = 20;
  input.placeholder = "Your name";
  input.style.cssText = "width:100%;padding:0.8rem;border-radius:12px;border:2px solid #94a3b8;background:#0f172a;color:#f8fafc;font-family:inherit";

  const submit = document.createElement("button");
  submit.className = "btn btn--primary";
  submit.textContent = "Submit score";

  const hint = document.createElement("p");
  hint.className = "leaderboard__hint";
  hint.hidden = true;

  const confirm = document.createElement("p");
  confirm.className = "leaderboard__confirm";
  confirm.hidden = true;

  function markSubmitted(name, message) {
    submittedName = name;
    state.submittedName = name;
    writeState("daily", state);
    // The form and the big score step aside — the board is the focus.
    document.getElementById("final-score").hidden = true;
    document.getElementById("score-label").hidden = true;
    document.getElementById("end-detail").hidden = true;
    row.hidden = true;
    confirm.textContent = message;
    confirm.hidden = false;
  }

  submit.addEventListener("click", async () => {
    const name = input.value.trim();
    if (!name) {
      // Client-side validation: no empty submissions.
      hint.textContent = "Please enter your name first.";
      hint.hidden = false;
      input.focus();
      return;
    }
    hint.hidden = true;
    submit.disabled = true;
    input.disabled = true;
    try {
      await api.daily.submitLeaderboard(name);
      markSubmitted(name, "Score submitted 🎉");
      await refreshList();
    } catch (err) {
      if (/already submitted/i.test(err.message)) {
        // This player is already on the board (e.g. submitted from
        // another tab) — show the board instead of the form.
        markSubmitted(name, "Your score is already on the board.");
        await refreshList();
        return;
      }
      submit.textContent = err.message;
      submit.disabled = false;
      input.disabled = false;
    }
  });

  const heading = document.createElement("h2");
  heading.className = "leaderboard__title";
  heading.textContent = "Leaderboard";

  const row = document.createElement("div");
  row.className = "leaderboard__form";
  row.appendChild(input);
  row.appendChild(submit);
  row.appendChild(hint);

  const list = document.createElement("ol");
  list.className = "leaderboard__list";

  // Rank badges: medals for the podium, subtle numbered chips for 4-10
  // (ten medals in a row would be noise).
  const rankHtml = (rank) => {
    if (rank === 1) return `<span class="leaderboard__rank rank-1">🥇</span>`;
    if (rank === 2) return `<span class="leaderboard__rank rank-2">🥈</span>`;
    if (rank === 3) return `<span class="leaderboard__rank rank-3">🥉</span>`;
    return `<span class="leaderboard__rank">${rank}</span>`;
  };

  const ownRow = (e) => {
    const li = document.createElement("li");
    li.className = "leaderboard__row me without-rank";
    li.innerHTML = `<span class="leaderboard__name"></span>` +
      `<span class="leaderboard__score">${e.score} pts</span>`;
    li.querySelector(".leaderboard__name").textContent = e.name;
    return li;
  };

  async function refreshList() {
    const status = await api.daily.status();
    list.innerHTML = "";
    const entries = status.leaderboard || [];
    if (entries.length === 0 && !submittedName) {
      const li = document.createElement("li");
      li.className = "leaderboard__empty";
      li.textContent = "No submissions yet — be the first!";
      list.appendChild(li);
      return;
    }
    let mineOnBoard = false;
    // Always render 10 rows: empty rows keep the board's shape.
    for (let i = 0; i < 10; i++) {
      const e = entries[i];
      const li = document.createElement("li");
      li.className = "leaderboard__row";
      if (!e) {
        // Empty slot: dimmed number chip only — no medal until the rank
        // is claimed, and no placeholder text.
        li.classList.add("empty");
        li.innerHTML = `<span class="leaderboard__rank">${i + 1}</span>`;
        list.appendChild(li);
        continue;
      }
      const mine = submittedName && e.name.toLowerCase() === submittedName.toLowerCase();
      if (mine) {
        mineOnBoard = true;
        li.classList.add("me");
      }
      li.innerHTML = rankHtml(i + 1) +
        `<span class="leaderboard__name"></span>` +
        `<span class="leaderboard__score">${e.score} pts</span>`;
      li.querySelector(".leaderboard__name").textContent = e.name;
      list.appendChild(li);
    }
    // Player submitted but is ranked below the visible top 10: append
    // their entry after the 10th row, without a rank number.
    if (submittedName && !mineOnBoard) {
      const mine = entries.find((e) => e.name.toLowerCase() === submittedName.toLowerCase());
      list.appendChild(ownRow(mine ?? { name: submittedName, score: serverScore }));
    }
  }

  // Form first, then the Leaderboard title over its list.
  section.appendChild(row);
  section.appendChild(heading);
  section.appendChild(list);
  root.appendChild(section);
  await refreshList();

  // Reopening the board after submitting (backend flag, or a name this
  // browser submitted earlier): the form and the big score step aside
  // immediately — the board is all that's left to show.
  if (alreadySubmitted && !submittedName) {
    submittedName = "You";
  }
  if (alreadySubmitted || submittedName) {
    markSubmitted(submittedName, "Your score is already on the board.");
  }
}

// showEnd swaps the game view for the end view and loads the result.
// Called by game.js at the end of the run via dynamic import.
export async function showEnd() {
  document.getElementById("landing-view").hidden = true;
  document.getElementById("game-view").hidden = true;
  document.getElementById("end-view").hidden = false;

  // The end-view main IS the card (no nested end-root container).
  const root = document.getElementById("end-view");
  // Reset dynamic content (leaderboard UI, back button) from any
  // previous run, keeping the static score/detail nodes.
  // .card-close is static markup and must survive the cleanup.
  root.querySelectorAll("input, button:not(.card-close), ol, a, .leaderboard").forEach((n) => n.remove());

  const leaveEnd = () => {
    // Keep game:<mode> (finished=true) — the landing page uses it to
    // offer the Leaderboard button. Mark the deliberate exit so a
    // refresh lands on the landing page, not back on the board.
    setActiveMode(m, true);
    document.getElementById("end-view").hidden = true;
    document.getElementById("landing-view").hidden = false;
    window.dispatchEvent(new CustomEvent("game:exit"));
  };
  document.querySelector("#end-view .card-close")?.addEventListener("click", leaveEnd);

  const m = activeMode();
  const game = readState(m) || {};
  const fallbackScore = Number(game.score || 0);
  try {
    if (m === "daily") {
      const status = await api.daily.status();
      const res = await api.daily.result();
      renderEndScreen(res);
      await renderLeaderboard(root, res.score, status.submitted);
    } else {
      const res = await api.freeplay.result(game.session);
      renderEndScreen(res);
    }
  } catch (err) {
    // Result endpoint failure: show the locally tracked score so the
    // flow still completes.
    renderEndScreen({ score: fallbackScore });
    const detail = document.getElementById("end-detail");
    detail.textContent = err.message;
    console.error(err);
  }
}
