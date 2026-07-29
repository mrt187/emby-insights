# Produkt-Blueprint — Emby Insights

## Produktversprechen

**Dein persönlicher Überblick über deine Mediathek.**

Emby Insights ist keine Player-Oberfläche, kein Server-Monitor und kein Tracearr-Klon. Es ist eine Companion-App für jeden Nutzer eines Emby-Servers: persönlich, visuell und auf Smartphone sowie Tablet zu Hause.

## Zielgruppe

Normale Emby-Nutzer in einem privaten oder Familien-Server. Administratoren konfigurieren die Datenquellen und Nutzerzuordnungen, benutzen das Produkt aber nicht als Hauptzielgruppe.

## Abgrenzung

| Gehört dazu | Gehört bewusst nicht dazu |
| --- | --- |
| Persönliche Sehstatistiken | Filme oder Serien abspielen |
| Eigene Seerr-Anfragen | Server-, CPU- oder Docker-Monitoring |
| Persönlicher Coming-Soon-Kalender | Vollständige Tracearr-Administration |
| Persönlicher Feed und Benachrichtigungen | Emby-Webclient ersetzen |
| Mobile-first Dashboard | „Weiterschauen“-Reihe |

„Weiterschauen“ bleibt in Emby, wo Wiedergabe stattfindet.

## MVP (v0.1)

1. Anmeldung ausschließlich über „Mit Emby anmelden“ mit den bestehenden Emby-Zugangsdaten. Die App prüft die Daten gegen Emby, speichert keine Passwörter und wechselt nicht zur Emby-Weboberfläche.
2. Persönliche Home-Ansicht mit Begrüßung und wenigen klaren Karten.
3. Persönliche Wochen-/Monatsstatistiken aus Emby und Playback Reporting.
4. Eigene Anfragen und deren verständlicher Status aus Seerr.
5. Persönliche Ansicht kommender Veröffentlichungen aus Radarr, Sonarr und TMDB.
6. Benachrichtigungen über eine anklickbare Glocke mit Anzahl ungelesener Nachrichten. Quellen: Seerr, Release-Kalender und Emby; Beispiele sind „Anfrage genehmigt“, „Neue Folge erscheint morgen“ oder „Deine Anfrage ist jetzt in Emby verfügbar“. Eine Verfügbarkeits-Benachrichtigung erhält nur der Nutzer, der den Titel angefragt hat. Beim Öffnen werden sie als gelesen markiert.
7. Responsive Oberfläche: Smartphone zuerst, Tablet optimiert, Desktop ergänzt.
8. Serverseitige Konfiguration der Verbindungen über eine `.env`-Datei; API-Schlüssel werden niemals an den Browser übertragen.

## Home

Die Startseite beantwortet vier Fragen: Was ist neu für mich? Was passiert mit meinen Anfragen? Was kommt bald? Wie war meine Medienwoche?

Vorgesehene Karten:

- **Willkommen** — Tageszeit, Name, kurze persönliche Zusammenfassung.
- **Meine Woche** — Datenquelle: Emby und Playback Reporting. Sehzeit, gesehene Filme, vollständig abgeschlossene Serien und Lieblingsgenre; Zeitraum zwischen Woche, Monat und Jahr umschaltbar.
- **Demnächst** — Datenquellen: Radarr, Sonarr und optional TMDB für regional korrekte Filmtermine. Die einfache, nicht nutzerbezogene Startseiten-Karte zeigt die nächsten vier Wochen als horizontal scrollbare Poster-Reihe; darunter steht „Verfügbar in …“. Eine separate Reihe **Im Kino** zeigt kommende Kinostarts sowie Filme zwischen regionalem Kinostart und ihrer digitalen Verfügbarkeit.
- **Meine Anfragen** — Datenquelle: Seerr. Horizontal scrollbare Poster-Reihe mit ausschließlich offenen oder noch nicht verfügbaren Anfragen. Der Status steht als verständlicher Text unter dem Poster, z. B. angefragt, genehmigt oder in Bearbeitung.
- **Neu für dich** — Datenquelle: Emby-API. 15 Poster in einer horizontal scrollbaren Reihe. Gezeigt werden Inhalte, die in den letzten 14 Tagen zur Bibliothek hinzugefügt wurden und die der jeweilige Nutzer noch nicht gesehen hat.

## Navigation

Mobil: feste untere Navigation mit **Heute**, **Statistik**, **Anfragen** und **Profil**. Auf größeren Ansichten wird daraus eine schmale linke Navigation.

## Profil (v0.1)

Das Profil enthält nur Anzeigename, Avatar und die Möglichkeit zum Abmelden.

## Designprinzipien

- Dark-first, aber nicht technisch oder überladen.
- Große Poster, großzügige Abstände, kurze Texte.
- Emby-Grün als primärer Akzent; Blau für Statistik, warmes Orange für „Demnächst“ und Rot nur für problematische Anfrage-Status.
- Bewegungen dezent und funktional: Übergänge, Statuswechsel, Karten-Reihenfolge.
- Module können später pro Nutzer ein- und ausgeblendet sowie sortiert werden. Für v0.1 ist die Reihenfolge fest.

## Nicht im MVP

- Medien direkt anfragen
- Push-Nachrichten
- Achievements, Ziele, Wrapped
- Social- oder Ranking-Funktionen
- Mehrere Medienserver
- frei konfigurierbare Widgets und persönliche Reihenfolge der Dashboard-Karten
- eigene Startseiten-Karte für angefangene, noch nicht vollständig angesehene Inhalte
- Nutzer können dem Server-Administrator Nachrichten senden
- Bereich „Im Trend“ prüfen
- Detailansicht für Medien mit Beschreibung, Cast und Trailer
- Mobile Push-Benachrichtigungen

Diese Funktionen sind bewusst nachgelagert, damit die erste Version täglich nutzbar und zuverlässig bleibt.

## Statistik (v0.1)

Die Statistik-Seite zeigt für den ausgewählten Zeitraum (Woche, Monat oder Jahr):

- Sehzeit
- gesehene Filme
- vollständig abgeschlossene Serien
- Lieblingsgenre
- Veränderung zum vorherigen gleich langen Zeitraum

Datenquelle: Emby und Playback Reporting.
