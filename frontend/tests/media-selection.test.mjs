import assert from "node:assert/strict";
import test from "node:test";
import { calendarSelection } from "../app/media-selection.ts";

const movie = { id: "7", tmdbId: "7", source: "radarr", detailId: "7", mediaType: "movie" };
// Ohne TMDB liefert Sonarr nur eine TVDB-Id — tmdbId bleibt leer.
const series = { id: "42", tmdbId: "", source: "sonarr", detailId: "42", mediaType: "tv" };
const seriesWithTmdb = { id: "99", tmdbId: "99", source: "sonarr", detailId: "42", mediaType: "tv" };

test("mit Seerr und TMDB-Id gewinnt Seerr — dessen Detail hat Besetzung und Staffeln", () => {
  assert.deepEqual(calendarSelection(movie, true), { source: "seerr", id: "7", mediaType: "movie" });
  assert.deepEqual(calendarSelection(seriesWithTmdb, true), { source: "seerr", id: "99", mediaType: "tv" });
});

test("ohne Seerr immer der Kalender-Screen", () => {
  assert.deepEqual(calendarSelection(movie, false), {
    source: "comingsoon", id: "7", mediaType: "movie", via: "radarr", tmdbId: "7",
  });
  assert.deepEqual(calendarSelection(series, false), {
    source: "comingsoon", id: "42", mediaType: "tv", via: "sonarr", tmdbId: undefined,
  });
});

// Der Fall, der beide Wege braucht: Seerr laeuft, aber TMDB nicht. Sonarrs
// TVDB-Id an Seerr zu schicken wuerde den falschen Titel oeffnen.
test("Seerr ohne TMDB-Id wird nicht benutzt", () => {
  const selection = calendarSelection(series, true);
  assert.equal(selection.source, "comingsoon");
  assert.equal(selection.id, "42");
  assert.equal(selection.via, "sonarr");
});

test("die Id faellt sinnvoll zurueck", () => {
  const ohneDetailId = { id: "5", tmdbId: "", source: "radarr", detailId: "", mediaType: "movie" };
  assert.equal(calendarSelection(ohneDetailId, false).id, "5");
  const nurTmdb = { id: "5", tmdbId: "8", source: "radarr", detailId: "", mediaType: "movie" };
  assert.equal(calendarSelection(nurTmdb, false).id, "8");
});
