<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { Download, Share, X } from 'lucide-svelte';
  import { pwa } from '$lib/stores/pwa.svelte';

  /**
   * Der Vorschlag, die Seite als App zu installieren. Das Abfangen des
   * Installationsangebots übernimmt der pwa-Store (er lauscht von Anfang an);
   * hier geht es nur um das Anzeigen — und darum, nach einem „Später" einen
   * Monat Ruhe zu geben. Wer es sich anders überlegt, findet die Installation
   * unter Einstellungen → App.
   */
  const DISMISS_KEY = 'install-prompt-dismissed';

  let zurueckgestellt = $state(true);
  let iosHinweis = $state(false);

  const sichtbar = $derived(
    !zurueckgestellt && !pwa.installiert && (pwa.installierbar || (pwa.iosSafari && iosHinweis)),
  );

  function dismiss() {
    zurueckgestellt = true;
    try {
      localStorage.setItem(DISMISS_KEY, String(Date.now()));
    } catch {
      /* private mode */
    }
  }

  async function install() {
    await pwa.installieren();
    dismiss();
  }

  onMount(() => {
    // Gefragt und abgelehnt innerhalb des letzten Monats: still bleiben.
    try {
      const dismissed = Number(localStorage.getItem(DISMISS_KEY) ?? 0);
      zurueckgestellt = Date.now() - dismissed < 30 * 24 * 60 * 60 * 1000;
    } catch {
      zurueckgestellt = false;
    }

    // iOS kennt kein Installationsfenster; Safari braucht einen Hinweis zum
    // Selbermachen. Erst nach einer Weile, damit man das Dashboard erst
    // einmal ansehen kann.
    if (pwa.iosSafari) {
      const timer = setTimeout(() => (iosHinweis = true), 8000);
      return () => clearTimeout(timer);
    }
  });
</script>

{#if sichtbar}
  <div
    class="safe-bottom fixed inset-x-3 bottom-[calc(var(--unten-leiste,0px)+1rem)] z-50 mx-auto max-w-md animate-slide-up rounded-2xl border border-border bg-card p-4 shadow-xl"
  >
    <div class="flex items-start gap-3">
      <span class="text-2xl">🏠</span>
      <div class="min-w-0 flex-1">
        <p class="text-sm font-semibold">{t('Als App installieren')}</p>
        {#if pwa.installierbar}
          <p class="mt-0.5 text-xs text-muted-foreground">
            {t('Öffnet ohne Browserleiste und startet schneller.')}
          </p>
        {:else}
          <p class="mt-0.5 text-xs text-muted-foreground">
            {t('In Safari auf')} <Share class="inline h-3 w-3" /> {t('tippen, dann „Zum Home-Bildschirm".')}
          </p>
        {/if}
      </div>
      <button class="touch-target shrink-0 text-muted-foreground" onclick={dismiss} aria-label={t('Schließen')}>
        <X class="h-4 w-4" />
      </button>
    </div>

    {#if pwa.installierbar}
      <div class="mt-3 flex gap-2">
        <button class="btn-outline flex-1 text-sm" onclick={dismiss}>{t('Später')}</button>
        <button class="btn-primary flex-1 text-sm" onclick={install}>
          <Download class="h-4 w-4" /> {t('Installieren')}
        </button>
      </div>
    {/if}
  </div>
{/if}
