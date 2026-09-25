// End view: final score, per-type breakdown, daily top-10 (Daily only),
// and return-to-main-menu. Rendered as a view inside "/" — there is no
// /end URL and no dedicated replay control, per the PRD.
import { api } from "./api.js";

// The mode whose run just finished (recorded by game.js in game:active).
function activeMode() {
  const a = JSON.parse(localStorage.getItem("game:active") || "null");
  return a ? a.mode : "daily";
}
function readState(m) {
  return JSON.parse(localStorage.getItem(`game:${m}`) || "null");
}

export function renderEndScreen({ score, breakdown, leaderboard }) {
  const scoreEl = document.getElementById("final-score");
  const detail = document.getElementById("end-detail");
  scoreEl.textContent = `${score} / 100`;

  if (breakdown) {
    detail.textContent = Object.entries(breakdown)
      .map(([type, pts]) => `${type}: ${pts}`)
      .join(" · ");
  }

  if (leaderboard) {
    renderLeaderboard(leaderboard);
  }
}

function renderLeaderboard(entries) {
  const root = document.getElementById("end-root");
  const input = document.createElement("input");
  input.type = "text";
  input.maxLength = 20;
  input.placeholder = "Your name for the leaderboard";
  input.style.cssText = "width:100%;padding:0.8rem;border-radius:12px;border:2px solid #94a3b8;background:#0f172a;color:#f8fafc;font-family:inherit";
  const submit = document.createElement("button");
  submit.className = "btn btn--primary";
  submit.textContent = "Submit score";
  submit.addEventListener("click", async () => {
    submit.disabled = true;
    try {
      await api.daily.submitLeaderboard(input.value.trim());
      submit.textContent = "Submitted";
      input.disabled = true;
    } catch (err) {
      submit.textContent = err.message;
    }
  });
  root.appendChild(input);
  root.appendChild(submit);

  const list = document.createElement("ol");
  list.style.textAlign = "left";
  entries.forEach((e) => {
    const li = document.createElement("li");
    li.textContent = `${e.name} — ${e.score}`;
    list.appendChild(li);
  });
  root.appendChild(list);
}

// showEnd swaps the game view for the end view and loads the result.
// Called by game.js at the end of the run via dynamic import.
export async function showEnd() {
  document.getElementById("landing-view").hidden = true;
  document.getElementById("game-view").hidden = true;
  document.getElementById("end-view").hidden = false;

  const root = document.getElementById("end-root");
  // Reset dynamic content (leaderboard UI, back button) from any
  // previous run, keeping the static score/detail nodes.
  root.querySelectorAll("input, button, ol, a").forEach((n) => n.remove());

  document.querySelector("#end-view .card-close")?.addEventListener("click", () => {
    localStorage.removeItem(`game:${activeMode()}`);
    localStorage.removeItem("game:active");
    document.getElementById("end-view").hidden = true;
    document.getElementById("landing-view").hidden = false;
    window.dispatchEvent(new CustomEvent("game:exit"));
  });

  const back = document.createElement("a");
  back.className = "btn btn--outline";
  back.href = "/";
  back.textContent = "Return to main menu";
  back.addEventListener("click", (e) => {
    e.preventDefault();
    localStorage.removeItem(`game:${activeMode()}`);
    localStorage.removeItem("game:active");
    document.getElementById("end-view").hidden = true;
    document.getElementById("landing-view").hidden = false;
    window.dispatchEvent(new CustomEvent("game:exit"));
  });
  root.appendChild(back);

  const m = activeMode();
  const game = readState(m) || {};
  const fallbackScore = Number(game.score || 0);
  try {
    const res = m === "daily"
      ? await api.daily.result()
      : await api.freeplay.result(game.session);
    renderEndScreen(res);
  } catch (err) {
    // Result endpoint not implemented yet (step #6): show the locally
    // tracked score so the flow remains testable.
    renderEndScreen({ score: fallbackScore });
    const detail = document.getElementById("end-detail");
    detail.textContent = err.message;
    console.error(err);
  }
}
