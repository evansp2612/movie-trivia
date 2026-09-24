// End screen: final score, per-type breakdown, daily top-10 (Daily only),
// and return-to-main-menu. The game always returns to the landing screen;
// there is no dedicated replay control.
export function renderEndScreen({ score, breakdown, leaderboard }) {
  const root = document.body;
  root.innerHTML = "";
  const scoreEl = document.createElement("h1");
  scoreEl.textContent = `${score} / 100`;
  root.appendChild(scoreEl);
  // TODO: render breakdown by question type.
  if (leaderboard) {
    // TODO: name input + submit (one-shot), then ordered top-10 list.
  }
}
