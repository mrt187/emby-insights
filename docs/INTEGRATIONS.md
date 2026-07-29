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

Tracearr wird nicht benötigt. Ein kleiner, regulärer Emby-Connector liest Playback Reporting ausschließlich lesend und stellt Emby Insights die erforderlichen, nutzerbezogenen Statistikdaten bereit. Der Container erhält keinen direkten Datenbankzugriff auf Emby-Appdata.

**Für „Neu für dich“**

Die API liefert bereits ungesehene, neueste Inhalte pro Nutzer. Der exakte 14-Tage-Filter wird gegen die laufende Emby-Instanz getestet; falls nötig, filtert das Backend die abgefragten Ergebnisse anhand des Hinzufügedatums.

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

## Benötigte Umgebungsvariablen

```text
EMBY_URL=
EMBY_API_KEY=
SEERR_URL=
SEERR_API_KEY=
```

Für abweichende Zuordnungen werden zusätzlich optionale Mapping-Werte ergänzt.
