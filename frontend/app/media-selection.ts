// Welcher Detailscreen zu einem Kalendereintrag gehoert. Eigenes Modul, damit
// die Entscheidung testbar ist, ohne die Seite zu rendern.

export type MediaSelection =
  | { source: "emby"; id: string }
  | { source: "seerr"; id: string; mediaType: string }
  | { source: "comingsoon"; id: string; mediaType: string; via: "radarr" | "sonarr"; tmdbId?: string };

export type CalendarItem = {
  id: string;
  tmdbId: string;
  source: "radarr" | "sonarr";
  detailId: string;
  mediaType: string;
};

// Mit Seerr ist dessen Detail das reichere: Besetzung, Crew und Staffeln
// liefern Radarr und Sonarr nicht. Voraussetzung ist eine echte TMDB-Id — ohne
// TMDB gibt Sonarr nur eine TVDB-Id her, mit der Seerr nichts anfangen kann.
// Sonst der Kalender-Screen, der ohne beides auskommt.
export function calendarSelection(item: CalendarItem, seerrConfigured: boolean): MediaSelection {
  if (seerrConfigured && item.tmdbId) {
    return { source: "seerr", id: item.tmdbId, mediaType: item.mediaType };
  }
  return {
    source: "comingsoon",
    id: item.detailId || item.tmdbId || item.id,
    mediaType: item.mediaType,
    via: item.source,
    tmdbId: item.tmdbId || undefined,
  };
}
