# Integrationsprüfung

Stand: 28. Juli 2026

## Emby

**Verifiziert**

- Nutzeranmeldung über `POST /Users/AuthenticateByName`.
- Serverseitige Integration über einen statischen API-Schlüssel.
- Neue, ungesehene Inhalte pro Nutzer über `GET /Users/{UserId}/Items/Latest` mit `IsPlayed=false`.
- Wiedergabestatus, Genre und der Abschluss von Filmen beziehungsweise Serien können über die Emby-Metadaten und Nutzerstatusdaten ermittelt werden.

## Playback Reporting

**Verifiziert auf dem laufenden Emby-Server**

- Das Plugin speichert je Wiedergabe Nutzer-ID, Item-ID, Medientyp, Datum, Wiedergabedauer, Client und Gerät.
- Die Daten liegen in einer eigenen SQLite-Datenbank und gehören nicht zur normalen Emby-REST-API.

**Folgerung**

Für die Basis-Statistiken wird Tracearr nicht benötigt. Ein kleiner, regulärer Emby-Connector liest Playback Reporting ausschließlich lesend und stellt Emby Insights die erforderlichen, nutzerbezogenen Statistikdaten bereit. Der Container erhält keinen direkten Datenbankzugriff auf Emby-Appdata. Das Plugin bleibt Pflichtquelle — siehe „Tracearr (optional)" für die Daten, die Playback Reporting nicht erhebt.

**Für „Neu für dich“**

Die API liefert bereits ungesehene, neueste Inhalte pro Nutzer. Der exakte 14-Tage-Filter wird gegen die laufende Emby-Instanz getestet; falls nötig, filtert das Backend die abgefragten Ergebnisse anhand des Hinzufügedatums.

## Tracearr (optional)

**Verifiziert gegen die laufende Instanz (09.08.2026)**

Tracearr ist eine zusätzliche, abschaltbare Quelle. Es ersetzt den Emby-Connector nicht und dient auch nicht als Ersatz, wenn dieser fehlt: Wiedergabezeit, Geräte, Stunden und Wochentage kommen weiterhin aus Playback Reporting. Tracearr erhebt aber Daten, die Playback Reporting gar nicht kennt.

**Anbindung**

- Public API v2 unter `{Basis-URL}/api/v2/public/…`, Authentifizierung per `Authorization: Bearer <API-Key>`.
- Den API-Key erzeugt man in der Tracearr-Oberfläche unter Settings; die OpenAPI-Spezifikation liegt live unter `GET /api/v2/public/docs`.
- Basis-URL und Key werden in der Verwaltung hinterlegt, der Key verschlüsselt gespeichert.

**Nutzer-Zuordnung**

Tracearr speichert die Nutzer-ID des Medienservers unverändert am verknüpften Konto (`accounts[].external_user_id` bei `server_type: "emby"`). Die Emby-Nutzer-ID aus der Session genügt also zur Zuordnung — es braucht keine Mapping-Tabelle und keine manuelle Verknüpfung. Das Ergebnis wird 24 Stunden gecacht, da sich die Zuordnung nur bei Konto-Änderungen am Server ändert.

**Genutzte Endpunkte und Daten**

- `GET /users` — Identitäten samt verknüpfter Server-Konten, für die Zuordnung oben.
- `GET /users/{id}/stats` — `top_genres` (Genre + Anzahl Wiedergaben). Playback Reporting liefert nur ein einzelnes Lieblingsgenre.
- `GET /history?user_id=…&watched=false&since=…` — abgebrochene Titel über `percent_complete`; gezählt wird zwischen 5 % und 90 %, je Titel einmal.
- `GET /history?user_id=…&since=…` — Transcode-Anteil über `is_transcode`.
- `GET /media/{ref}/watchers` und `GET /media/{ref}/stats` — Mitseher im Haushalt und Wiedergabezahl je Titel.

`ref` ist typqualifiziert: `{movie|show|episode}:{imdb|tmdb|tvdb}:{id}`, zum Beispiel `movie:tmdb:584`. Staffeln haben keine Provider-Referenz. Die dafür nötigen externen IDs kommen aus Embys `ProviderIds` beziehungsweise aus der TMDB-ID, mit der der Seerr-Detailscreen ohnehin aufgerufen wird.

**Bewusst nicht genutzt**

Geodaten pro Session, Violations und Trust-Score bleiben außen vor. Sie sind Überwachungsdaten und widersprechen der Produktgrundlage. Ebenso ausgelassen: Library-Rollups und Live-Streams (Admin-Sicht, kein persönlicher Nutzen) sowie die serverübergreifende Emby-Jellyfin-Identität.

**Ausfallverhalten**

Alle Tracearr-Aufrufe sind Dekoration. Ist die Integration aus, nicht erreichbar oder antwortet sie mit einem Fehler, liefern die betroffenen Endpunkte eine leere Antwort mit Status 200 und der Rest der Oberfläche bleibt unverändert.

## Release calendar

**Verifiziert im Radarr-/Sonarr- und TMDB-Quellcode**

- Radarr liefert überwachte Filme einschließlich Kino- und Digitaltermin sowie TMDB-ID.
- Sonarr liefert künftige Episoden mit `airDateUtc` und der TVDB-ID der Serie.
- TMDB ergänzt regionale Filmtermine; für Deutschland werden Kino- und Digitaltermine verwendet. Eine TVDB-ID kann über TMDB in eine TMDB-ID aufgelöst werden.

**Folgerung**

Emby Insights liest den Kalender direkt und erzeugt keine Emby-Bibliotheken.

## Seerr

**Verifiziert**

- API-Key-Authentifizierung über `X-Api-Key`.
- Eigene Anfragen über `GET /user/{userId}/requests`.
- Webhooks können Ereignisse sofort an einen eigenen HTTP-Endpunkt senden.
- Seerr kann einen Emby-Nutzer über seine Emby-ID verknüpfen; der vorhandene Embyeerr-Code nutzt dafür `GET /user/jellyfin/{embyUserId}`.

**Folgerung**

Für Seerr verwenden wir zuerst die stabile Emby-ID-Verknüpfung. Der Benutzernamen-Abgleich bleibt nur ein Fallback.

## Echtzeit

- **Seerr:** Webhook an Emby Insights für Anfrage- und Verfügbarkeitsereignisse.
- **Release calendar:** Radarr-, Sonarr- und TMDB-Daten werden für 15 Minuten in Redis zwischengespeichert.
- **Emby:** Erfordert einen Test der verfügbaren Bibliotheksereignisse; andernfalls nutzt Emby Insights einen gezielten kurzen Abgleich.
- **Browser:** Das Go-Backend sendet neue Daten sofort über eine eigene Live-Verbindung an geöffnete Dashboards.

## Konfiguration

Nur die Basis-Anbindung an Emby selbst (Adresse, Admin-API-Key) sowie die
Datenbank-/Verschlüsselungswerte stehen noch in `.env`. Radarr, Sonarr, TMDB,
Seerr, OMDb und Tracearr werden nicht über Umgebungsvariablen konfiguriert, sondern über
die Verwaltungsoberfläche (siehe Einrichtungsassistent): der erste Emby-Login
wird automatisch zum Emby-Insights-Admin und richtet die übrigen Dienste dort
ein.
