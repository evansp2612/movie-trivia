import { api, APIError } from "./api.js";
import { showGame } from "./game.js";

const dailyBtn = document.getElementById("play-daily");
const freeplayBtn = document.getElementById("play-freeplay");
const meta = document.getElementById("daily-meta");

function setDailyCompleted(completed) {
  dailyBtn.disabled = completed;
  dailyBtn.querySelector(".btn__check").hidden = !completed;
  dailyBtn.querySelector(".btn__label").textContent = completed
    ? "Completed today"
    : "Play Game of the Day";
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

// Game state lives in sessionStorage, not the URL: the address bar stays
// clean, session IDs can't be casually read or edited there, and the whole
// game runs as views of "/" — the only player-facing URL (plus /admin).
function startGame(mode, session) {
  sessionStorage.setItem("game", JSON.stringify({
    mode,
    session: session.id,
    round: session.current_round,
    score: 0,
  }));
  showGame(true);
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
  try {
    const session = await api.freeplay.start();
    startGame("freeplay", session);
  } catch (err) {
    meta.textContent = err.message;
    freeplayBtn.disabled = false;
    console.error(err);
  }
});

// Returning from a finished game (end.js "Return to main menu"):
// refresh the daily status so the completed state shows.
window.addEventListener("game:exit", loadStatus);

loadStatus();
