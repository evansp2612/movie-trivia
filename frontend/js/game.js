// Round rendering and answer submission for both modes.
// TODO: Higher/Lower card tap + reveal badges (Emerald/Rose borders),
// Blurred Poster search with 5-attempt dots and blur step-down,
// Guess the Year slider with floating gold year label.
export function renderRound(round) {
  document.body.dataset.roundType = round.type;
  // TODO: branch on round.type: higher_lower | blurred_poster | guess_the_year
  return round;
}

export async function submitAnswer(api, mode, sessionId, roundIndex, guess) {
  if (mode === "daily") {
    return api.daily.answer(roundIndex, guess);
  }
  return api.freeplay.answer(sessionId, roundIndex, guess);
}
