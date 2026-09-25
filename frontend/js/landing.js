import { api, APIError } from "./api.js";
import { showGame } from "./game.js";

const dailyBtn = document.getElementById("play-daily");
const freeplayBtn = document.getElementById("play-freeplay");
const meta = document.getElementById("daily-meta");

// The daily run is finished (kept in localStorage with finished=true
// after the end screen) → the GOTD button becomes a Leaderboard button
// so the player can submit or view their result. Whether they already
// submitted or not doesn't matter: the board page handles both.
function dailyFinished() {
  const saved = JSON.parse(localStorage.getItem("game:daily") || "null");
  return !!(saved && saved.finished);
}

function setDailyCompleted(finished) {
  if (finished) {
    dailyBtn.disabled = false;
    dailyBtn.querySelector(".btn__check").hidden = true;
    dailyBtn.querySelector(".btn__label").textContent = "Leaderboard";
  } else {
    dailyBtn.disabled = false;
    dailyBtn.querySelector(".btn__check").hidden = true;
    dailyBtn.querySelector(".btn__label").textContent = "Game of the Day";
  }
}

async function loadStatus() {
  setDailyCompleted(dailyFinished());
  try {
    await api.daily.status();
    meta.textContent = "";
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
  // Finished run → the button opens the leaderboard (score + top 10 +
  // submit form if the player hasn't submitted yet).
  if (dailyFinished()) {
    localStorage.setItem("game:active", JSON.stringify({ mode: "daily", exited: false }));
    const m = await import("./end.js");
    await m.showEnd();
    return;
  }
  dailyBtn.disabled = true;
  try {
    const session = await api.daily.start();
    startGame("daily", session);
  } catch (err) {
    if (err instanceof APIError && err.status === 409) {
      // Completed but the local flag was missing (e.g. played in
      // another tab): record it and offer the Leaderboard button.
      const saved = JSON.parse(localStorage.getItem("game:daily") || "null");
      if (saved) saved.finished = true;
      localStorage.setItem("game:daily", JSON.stringify(saved ?? { mode: "daily", finished: true }));
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

// Returning from a game or the leaderboard (✕ or back link): re-enable
// the start buttons and refresh the daily status / button label.
window.addEventListener("game:exit", () => {
  dailyBtn.disabled = false;
  freeplayBtn.disabled = false;
  loadStatus();
});

document.getElementById("hud-close")?.addEventListener("click", () => {
  window.gameShowMenu?.();
});

loadStatus();
