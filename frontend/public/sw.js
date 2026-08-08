// Minimal service worker for Web Push notifications only — no asset
// caching/offline support.

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

// A push message's payload is JSON built by the backend (see
// App.notifyPush in backend/internal/server/server.go): { title, body }.
// If parsing fails for any reason, fall back to a generic notification
// rather than silently dropping the push.
self.addEventListener("push", (event) => {
  let title = "Emby Insights";
  let body = "";
  try {
    const data = event.data ? event.data.json() : {};
    if (data.title) title = data.title;
    if (data.body) body = data.body;
  } catch {
    // Non-JSON payload — show a generic notification instead of throwing.
  }

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon: "/emby-insights-logo.png",
      badge: "/emby-insights-logo.png",
      data: { url: "/" },
    }),
  );
});

// Clicking a notification focuses an already-open tab if one exists,
// otherwise opens a new one — standard "bring the app to front" pattern.
self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const targetUrl = (event.notification.data && event.notification.data.url) || "/";

  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ("focus" in client) return client.focus();
      }
      if (self.clients.openWindow) return self.clients.openWindow(targetUrl);
    }),
  );
});
