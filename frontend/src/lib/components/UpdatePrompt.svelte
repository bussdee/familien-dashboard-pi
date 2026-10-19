<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { RefreshCw } from 'lucide-svelte';
  import { session } from '$lib/stores';

  /**
   * Eine neue Version der App übernimmt nicht von allein, solange jemand
   * etwas tut — sie wartet im Hintergrund (siehe service-worker.ts) und wird
   * zu einem Zeitpunkt freigegeben, an dem das Neuladen nichts kaputtmacht:
   *
   *  - auf Tipp („Neu laden"),
   *  - sobald die App in den Hintergrund geht oder das Handy gesperrt wird —
   *    das Neuladen merkt dann niemand, und beim nächsten Öffnen läuft die
   *    neue Version,
   *  - am Wandgerät nach zwei Minuten ohne Berührung: Dort drückt niemand
   *    auf „Neu laden", und ein Tablet, das wochenlang auf dem Stand vom
   *    ersten Tag stehenbleibt, ist genau die Klage, die hier behoben wird.
   */
  const RUHE_WANDGERAET_MS = 2 * 60_000;

  let waiting = $state<ServiceWorker | null>(null);

  function anwenden() {
    // Der Worker übernimmt, die Seite hört auf controllerchange und lädt neu.
    waiting?.postMessage('skip-waiting');
  }

  // Im Hintergrund freigeben.
  $effect(() => {
    if (!waiting) return;
    const bemerkt = () => document.visibilityState === 'hidden' && anwenden();
    document.addEventListener('visibilitychange', bemerkt);
    return () => document.removeEventListener('visibilitychange', bemerkt);
  });

  // Am Wandgerät nach einer Ruhepause freigeben; jede Berührung schiebt sie nach hinten.
  $effect(() => {
    if (!waiting || !$session.device) return;
    let timer: ReturnType<typeof setTimeout>;
    const neu = () => {
      clearTimeout(timer);
      timer = setTimeout(anwenden, RUHE_WANDGERAET_MS);
    };
    neu();
    window.addEventListener('pointerdown', neu);
    window.addEventListener('keydown', neu);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('pointerdown', neu);
      window.removeEventListener('keydown', neu);
    };
  });

  // onMount cannot both be async and return a cleanup, so the async work runs
  // in a nested call and the cleanup is held in a variable it can see.
  onMount(() => {
    if (!('serviceWorker' in navigator)) return;

    let aufraeumen: (() => void) | undefined;
    void setup().then((fn) => (aufraeumen = fn));
    return () => aufraeumen?.();
  });

  async function setup(): Promise<(() => void) | undefined> {
    // Beim allerersten Besuch übernimmt der Worker per clients.claim() — das
    // ist keine Aktualisierung, und ein Neuladen dort wäre überflüssig.
    const hatteWorker = !!navigator.serviceWorker.controller;
    let reloading = false;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (reloading || !hatteWorker) return;
      reloading = true;
      location.reload();
    });

    let registration: ServiceWorkerRegistration | undefined;
    try {
      registration = await navigator.serviceWorker.ready;
    } catch {
      return;
    }

    if (registration.waiting && navigator.serviceWorker.controller) waiting = registration.waiting;

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

    // Nachsehen, ob es etwas Neues gibt: stündlich (ein Wandgerät läuft
    // wochenlang) und jedes Mal, wenn die App wieder nach vorn kommt —
    // ein Handy, das über Nacht im Hintergrund lag, soll nicht bis zur
    // nächsten vollen Stunde auf dem alten Stand bleiben.
    const pruefen = () => void registration?.update().catch(() => {});
    const timer = setInterval(pruefen, 60 * 60 * 1000);
    const sichtbar = () => document.visibilityState === 'visible' && pruefen();
    document.addEventListener('visibilitychange', sichtbar);

    return () => {
      clearInterval(timer);
      document.removeEventListener('visibilitychange', sichtbar);
    };
  }
</script>

{#if waiting}
  <div
    class="safe-bottom fixed inset-x-3 bottom-[calc(var(--unten-leiste,0px)+1rem)] z-50 mx-auto flex max-w-md animate-slide-up items-center gap-3 rounded-2xl border border-border bg-card p-3 shadow-xl"
  >
    <RefreshCw class="h-5 w-5 shrink-0 text-primary" />
    <p class="min-w-0 flex-1 text-sm">
      <span class="font-medium">{t('Neue Version verfügbar')}</span>
      <span class="block text-xs text-muted-foreground">{t('Neu laden, um sie zu benutzen.')}</span>
    </p>
    <button class="btn-primary shrink-0 px-3 text-sm" onclick={anwenden}>{t('Neu laden')}</button>
  </div>
{/if}
