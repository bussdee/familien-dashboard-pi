/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />

import { build, files, version } from '$service-worker';

// SvelteKit hands us the exact list of hashed build assets, so the offline
// shell can never drift out of sync with what was deployed.
const CACHE = `family-dashboard-${version}`;

// The app shell itself is in neither `build` nor `files`, so without adding it
// explicitly the offline navigation fallback below would never find anything.
const SHELL = '/';
const PRECACHE = [...build, ...files, SHELL];

const worker = self as unknown as ServiceWorkerGlobalScope;

worker.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      // addAll rejects the whole batch if a single entry 404s; adding one by
      // one means a stale entry cannot leave the app with no cache at all.
      .then((cache) => Promise.allSettled(PRECACHE.map((url) => cache.add(url))))
      .then(() => worker.skipWaiting()),
  );
});

worker.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))))
      .then(() => worker.clients.claim()),
  );
});

// The page asks for the new version to take over after the user agrees.
worker.addEventListener('message', (event) => {
  if (event.data === 'skip-waiting') void worker.skipWaiting();
});

/** Read-only endpoints worth keeping for an offline glance at the dashboard. */
const CACHEABLE_API = ['/api/weather', '/api/calendar', '/api/chores', '/api/notes', '/api/scoreboard'];

worker.addEventListener('fetch', (event) => {
  const { request } = event;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.origin !== worker.location.origin) return;

  // Precached build assets: cache first, they are content-hashed.
  if (PRECACHE.includes(url.pathname)) {
    event.respondWith(
      caches.match(request).then((hit) => hit ?? fetch(request)),
    );
    return;
  }

  if (url.pathname.startsWith('/api/')) {
    const cacheable = CACHEABLE_API.some((path) => url.pathname.startsWith(path));
    if (!cacheable) return; // writes and live data always go to the network

    // Network first: family data must be fresh whenever the network allows,
    // and the copy is only there so an offline tablet is not blank.
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) {
            const copy = response.clone();
            void caches.open(CACHE).then((cache) => cache.put(request, copy));
          }
          return response;
        })
        .catch(async () => {
          const hit = await caches.match(request);
          if (hit) return hit;
          return new Response(JSON.stringify({ message: 'Offline – keine gespeicherten Daten' }), {
            status: 503,
            headers: { 'Content-Type': 'application/json' },
          });
        }),
    );
    return;
  }

  // Navigations: network first, fall back to the cached app shell.
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request).catch(async () => {
        const shell = await caches.match(SHELL);
        return (
          shell ??
          new Response('<h1>Offline</h1><p>Das Dashboard ist gerade nicht erreichbar.</p>', {
            status: 503,
            headers: { 'Content-Type': 'text/html; charset=utf-8' },
          })
        );
      }),
    );
    return;
  }

  event.respondWith(
    caches.match(request).then(
      (hit) =>
        hit ??
        fetch(request).then((response) => {
          if (response.ok && response.type === 'basic') {
            const copy = response.clone();
            void caches.open(CACHE).then((cache) => cache.put(request, copy));
          }
          return response;
        }),
    ),
  );
});

export {};
