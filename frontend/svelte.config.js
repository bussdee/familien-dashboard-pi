import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
export default {
  preprocess: vitePreprocess(),
  kit: {
    // Nach einem Update fragt die geöffnete Seite alle fünf Minuten nach, ob
    // es eine neue Fassung gibt. Fehlt eine alte Datei nach dem Update, lädt
    // SvelteKit dann die Seite neu, statt mit einem Fehler stehen zu bleiben.
    version: { pollInterval: 5 * 60 * 1000 },
    // The dashboard is a pure client-side app served by nginx; every route
    // falls back to index.html and talks to the Go API over /api.
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html',
      precompress: false,
      strict: false,
    }),
  },
};
