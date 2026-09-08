<script lang="ts">
  import { onMount } from 'svelte';
  import { RefreshCw } from 'lucide-svelte';

  let waiting = $state<ServiceWorker | null>(null);

  function reload() {
    // The worker calls clients.claim() after skipWaiting, and the
    // controllerchange listener below reloads once it has taken over.
    waiting?.postMessage('skip-waiting');
  }

  // onMount cannot both be async and return a cleanup, so the async work runs
  // in a nested call and the timer is held in a variable the cleanup can see.
  onMount(() => {
    if (!('serviceWorker' in navigator)) return;

    let timer: ReturnType<typeof setInterval> | null = null;
    void setup().then((handle) => (timer = handle ?? null));

    return () => {
      if (timer) clearInterval(timer);
    };
  });

  async function setup(): Promise<ReturnType<typeof setInterval> | undefined> {
    let reloading = false;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (reloading) return;
      reloading = true;
      location.reload();
    });

    let registration: ServiceWorkerRegistration | undefined;
    try {
      registration = await navigator.serviceWorker.ready;
    } catch {
      return;
    }

    if (registration.waiting) waiting = registration.waiting;

    registration.addEventListener('updatefound', () => {
      const installing = registration.installing;
      if (!installing) return;
      installing.addEventListener('statechange', () => {
        // "installed" with a controller present means an update is queued
        // behind the version currently running.
        if (installing.state === 'installed' && navigator.serviceWorker.controller) {
          waiting = installing;
        }
      });
    });

    // A wall tablet may run for weeks; check hourly so it does not fall behind.
    return setInterval(() => void registration?.update(), 60 * 60 * 1000);
  }
</script>

{#if waiting}
  <div
    class="safe-bottom fixed inset-x-3 bottom-20 z-50 mx-auto flex max-w-md animate-slide-up items-center gap-3 rounded-2xl border border-border bg-card p-3 shadow-xl md:bottom-4"
  >
    <RefreshCw class="h-5 w-5 shrink-0 text-primary" />
    <p class="min-w-0 flex-1 text-sm">
      <span class="font-medium">Neue Version verfügbar</span>
      <span class="block text-xs text-muted-foreground">Neu laden, um sie zu benutzen.</span>
    </p>
    <button class="btn-primary shrink-0 px-3 text-sm" onclick={reload}>Neu laden</button>
  </div>
{/if}
