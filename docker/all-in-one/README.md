# All-in-one-Container

Backend, PostgreSQL und Redis in einem Container mit einem persistenten
`/config`-Volume. Läuft auf jedem Linux-Host mit Docker.

## Start

```bash
cp .env.example .env
# .env ausfüllen, siehe Tabelle
docker compose up -d
```

## Variablen

| Variable | Bedeutung |
| --- | --- |
| `EMBY_INSIGHTS_IMAGE` | Pflicht, ohne Default. Registry-Image oder eigener Build-Tag. |
| `EMBY_INSIGHTS_PORT` | Port auf dem Host, Standard `8081`. |
| `LISTEN_ADDRESS` | Nur ändern, wenn Port 8080 im Container belegt ist. |
| `POSTGRES_PASSWORD` | Langer, eigener Wert. Nach dem ersten Start nicht mehr ändern, sonst ist die Datenbank unbrauchbar. |
| `APP_ENCRYPTION_KEY` | Einmalig mit `openssl rand -base64 32` erzeugen und stabil halten. Verschlüsselt die gespeicherten API-Keys von Seerr, Radarr, Sonarr und TMDB; geht er verloren, sind diese unlesbar. |
| `EMBY_BASE_URL` | Adresse des Emby-Servers, inklusive `/emby`. |
| `EMBY_ADMIN_API_KEY` | In Emby unter Dashboard → Erweitert → Sicherheit → API-Schlüssel anlegen. |
| `COOKIE_SECURE` | `false` nur für den ersten direkten HTTP-Test, danach `true`. |
| `TRUSTED_PROXIES` | Optional, kommagetrennte IPs oder CIDRs des vorgelagerten Reverse Proxy. Nur diese Quellen dürfen per `X-Forwarded-For` die echte Client-Adresse melden. Leer lassen, wenn der Container direkt erreicht wird. |

Alles Weitere — welche Dienste aktiv sind, deren Adressen und Keys, die
Bibliotheksauswahl — wird nach dem ersten Login in der Verwaltungsoberfläche
konfiguriert. Der erste Emby-Account, der sich erfolgreich anmeldet, wird
automatisch Administrator.

## Betrieb

Im Container lauscht die API auf Port `8080`:

- `GET /healthz` — Prozess läuft
- `GET /readyz` — PostgreSQL und Redis erreichbar

Emby-Passwörter werden nie gespeichert. Nach dem Login liegt nur der temporäre
Emby-Access-Token in Redis. Die Emby-Device-ID wird beim ersten Start erzeugt
und in PostgreSQL abgelegt.

## Unraid

Dieses Verzeichnis nach `/mnt/cache/appdata/emby-insights/` kopieren und über
den Compose Manager starten.

## Image selbst bauen

```bash
docker build -f docker/all-in-one/Dockerfile -t emby-insights:local .
```

Danach `EMBY_INSIGHTS_IMAGE=emby-insights:local` in der `.env` setzen.
