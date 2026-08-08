"use client";

import { useEffect } from "react";

// Registers /sw.js on every page load, independent of whether the user has
// opted into push notifications yet — the opt-in flow (see the Profile page)
// needs an active registration to call pushManager.subscribe() on. A no-op
// on browsers without support (older Safari, some in-app webviews).
export function ServiceWorkerRegistration() {
  useEffect(() => {
    if (!("serviceWorker" in navigator)) return;
    navigator.serviceWorker.register("/sw.js").catch(() => {
      // Registration can fail (e.g. insecure context in local dev over
      // plain HTTP) — push simply stays unavailable, nothing else breaks.
    });
  }, []);
  return null;
}
