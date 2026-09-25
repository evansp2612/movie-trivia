import { api, APIError } from "./api.js";
import { showGame } from "./game.js";

const dailyBtn = document.getElementById("play-daily");
const freeplayBtn = document.getElementById("play-freeplay");
const meta = document.getElementById("daily-meta");

function setDailyCompleted(completed) {
  dailyBtn.disabled = completed;
  dailyBtn.querySelector(".btn__check").hidden = !completed;
  dailyBtn.querySelector(".btn__label").textContent = completed
    ? "Completed"
    : "Game of the Day";
}

async function loadStatus() {
  try {
    const status = await api.daily.status();
    setDailyCompleted(status.is_completed);
    meta.textContent = "";
    if (status.is_completed && (status.leaderboard?.length ?? 0) > 0) {
      const best = status.leaderboard[0];
      meta.textContent = `Today's best: ${best.name} — ${best.score}/100`;
    }
  } catch (err) {
    meta.textContent = "Could not reach the server. Is the API running?";
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

dailyBtn.addEventListener("click", async () => {
  dailyBtn.disabled = true;
  try {
    const session = await api.daily.start();
    startGame("daily", session);
  } catch (err) {
    if (err instanceof APIError && err.status === 409) {
      setDailyCompleted(true);
    } else {
      meta.textContent = err.message;
    }
    dailyBtn.disabled = false;
    console.error(err);
  }
});

freeplayBtn.addEventListener("click", async () => {
  freeplayBtn.disabled = true;
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

// Returning from a game (✕ or end screen): re-enable the start buttons —
// the start handlers disable them and never run again while the game view
// is up — and refresh the daily status so the completed state shows.
window.addEventListener("game:exit", () => {
  dailyBtn.disabled = false;
  freeplayBtn.disabled = false;
  loadStatus();
});

document.getElementById("hud-close")?.addEventListener("click", () => {
  window.gameShowMenu?.();
});

loadStatus();
