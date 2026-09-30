import { api, APIError } from "./api.js";
import { showGame } from "./game.js";

const dailyBtn = document.getElementById("play-daily");
const freeplayBtn = document.getElementById("play-freeplay");
const meta = document.getElementById("daily-meta");

// The button label follows the SERVER's status (/api/daily/status →
// is_completed), never a stale localStorage flag: after midnight the
// server reports the new day as not completed, so the button flips back
// to "Game of the Day" and the player can play again.
let serverCompleted = false;

function setDailyCompleted(finished) {
  serverCompleted = finished;
  setDailyLoading(false); // normal face: label visible, spinner hidden
  dailyBtn.disabled = false;
  dailyBtn.querySelector(".btn__label").textContent = finished
    ? "Leaderboard"
    : "Game of the Day";
}

function clearStaleDailyState() {
  // Yesterday's finished run: the server says today is not completed,
  // so the saved state can only mislead (stale Leaderboard button,
  // stale auto-resume). An unfinished run (finished=false) is kept —
  // it resumes server-side via /daily/start.
  const saved = JSON.parse(localStorage.getItem("game:daily") || "null");
  if (saved && saved.finished) {
    localStorage.removeItem("game:daily");
  }
}

async function loadStatus() {
  // Spinner while checking — never paint "Game of the Day" first and
  // flash to "Leaderboard" when the status says otherwise.
  setDailyLoading(true);
  try {
    const status = await api.daily.status();
    clearStaleDailyState();
    setDailyCompleted(status.is_completed);
    meta.textContent = "";
  } catch (err) {
    // Offline: keep the button playable as a plain GOTD start.
    setDailyLoading(false);
    dailyBtn.disabled = false;
    dailyBtn.querySelector(".btn__label").textContent = "Game of the Day";
    meta.textContent = "Could not reach the server.";
    console.error(err);
  }
}

// Game state lives in localStorage (one key per mode), not the URL: the
// address bar stays clean, session IDs can't be casually read or edited
// there, and the whole game runs as views of "/" — the only player-facing
// URL (plus /admin).
function startGame(m, session) {
  localStorage.setItem(`game:${m}`, JSON.stringify({
    mode: m,
    session: session.id,
    round: session.current_round,
    score: 0,
    finished: false,
  }));
  showGame(m);
}

// Loading state for the GOTD button: hides the label/check and shows
// the spinner ring. restored() puts the normal label back.
const labelEl = dailyBtn.querySelector(".btn__label");
const checkEl = dailyBtn.querySelector(".btn__check");
const spinnerEl = dailyBtn.querySelector(".spinner");

function setDailyLoading(loading) {
  dailyBtn.disabled = loading;
  labelEl.hidden = loading;
  spinnerEl.hidden = !loading;
}

let dailyPollAborted = false;

dailyBtn.addEventListener("click", async () => {
  // Completed run (per the server) → the button opens the leaderboard
  // (score + top 10 + submit form if the player hasn't submitted yet).
  if (serverCompleted) {
    localStorage.setItem("game:active", JSON.stringify({ mode: "daily", exited: false }));
    const m = await import("./end.js");
    await m.showEnd();
    return;
  }
  setDailyLoading(true);
  try {
    const session = await startDailyWithRetry();
    setDailyLoading(false);
    startGame("daily", session);
  } catch (err) {
    if (err instanceof APIError && err.status === 409) {
      // Completed but the server flag hadn't reached this tab yet.
      setDailyCompleted(true);
    } else if (err.status !== 0) {
      meta.textContent = err.message;
    }
    setDailyLoading(false);
    console.error(err);
  }
});

// Right after midnight the daily chain (pool refresh + game generation)
// may still be running: /daily/start then answers 503 "preparing".
// Poll every 1s until the game is ready (~60s cap). Free Play during
// the poll aborts it cleanly — the daily session simply sits unfinished.
async function startDailyWithRetry(attempts = 60) {
  dailyPollAborted = false;
  for (let i = 0; i < attempts; i++) {
    if (dailyPollAborted) throw new APIError("aborted", 0);
    try {
      return await api.daily.start();
    } catch (err) {
      if (dailyPollAborted) throw new APIError("aborted", 0);
      if (err instanceof APIError && err.status === 503 && i < attempts - 1) {
        await new Promise((r) => setTimeout(r, 1000));
        continue;
      }
      throw err;
    }
  }
  throw new APIError("Game is taking longer to prepare — try again soon.", 503);
}

freeplayBtn.addEventListener("click", async () => {
  freeplayBtn.disabled = true;
  // A pending GOTD poll is abandoned — Free Play always wins.
  dailyPollAborted = true;
  // Resume an unfinished Free Play session if one is saved (e.g. the
  // player left via the ✕); only start a new variant otherwise.
  const saved = JSON.parse(localStorage.getItem("game:freeplay") || "null");
  if (saved && saved.session && !saved.finished) {
    freeplayBtn.disabled = false;
    showGame("freeplay");
    return;
  }
  try {
    const session = await api.freeplay.start();
    startGame("freeplay", session);
  } catch (err) {
    meta.textContent = err.message;
    freeplayBtn.disabled = false;
    console.error(err);
  }
});

// Returning from a game or the leaderboard (✕ or back link): re-enable
// the start buttons and refresh the daily status / button label.
window.addEventListener("game:exit", () => {
  dailyBtn.disabled = false;
  freeplayBtn.disabled = false;
  loadStatus();
});

// A tab left open across midnight: re-check the server status as soon
// as the player looks at the tab again, so the button flips back to
// "Game of the Day" without a manual refresh.
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) loadStatus();
});

document.getElementById("hud-close")?.addEventListener("click", () => {
  window.gameShowMenu?.();
});

loadStatus();
