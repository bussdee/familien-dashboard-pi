<script lang="ts">
  import { onMount } from 'svelte';
  import { Download, Share, X } from 'lucide-svelte';

  /** Chrome fires this instead of showing its own bar; we save it for later. */
  interface BeforeInstallPromptEvent extends Event {
    prompt(): Promise<void>;
    userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
  }

  const DISMISS_KEY = 'install-prompt-dismissed';

  let deferred = $state<BeforeInstallPromptEvent | null>(null);
  let showIosHint = $state(false);
  let visible = $state(false);

  function dismiss() {
    visible = false;
    try {
      localStorage.setItem(DISMISS_KEY, String(Date.now()));
    } catch {
      /* private mode */
    }
  }

  async function install() {
    if (!deferred) return;
    await deferred.prompt();
    await deferred.userChoice;
    deferred = null;
    dismiss();
  }

  onMount(() => {
    // Already installed? Nothing to offer.
    const standalone =
      window.matchMedia('(display-mode: standalone)').matches ||
      (navigator as { standalone?: boolean }).standalone === true;
    if (standalone) return;

    // Asked and declined within the last month: stay quiet.
    try {
      const dismissed = Number(localStorage.getItem(DISMISS_KEY) ?? 0);
      if (Date.now() - dismissed < 30 * 24 * 60 * 60 * 1000) return;
    } catch {
      /* private mode */
    }

    const onBeforeInstall = (event: Event) => {
      event.preventDefault();
      deferred = event as BeforeInstallPromptEvent;
      visible = true;
    };
    window.addEventListener('beforeinstallprompt', onBeforeInstall);

    // iOS has no install event; Safari needs a hand-written hint instead.
    const isIos = /iphone|ipad|ipod/i.test(navigator.userAgent);
    const isSafari = /^((?!chrome|android|crios|fxios).)*safari/i.test(navigator.userAgent);
    if (isIos && isSafari) {
      showIosHint = true;
      // Give people a moment with the dashboard before suggesting anything.
      setTimeout(() => (visible = true), 8000);
    }

    return () => window.removeEventListener('beforeinstallprompt', onBeforeInstall);
  });
</script>

{#if visible && (deferred || showIosHint)}
  <div
    class="safe-bottom fixed inset-x-3 bottom-20 z-50 mx-auto max-w-md animate-slide-up rounded-2xl border border-border bg-card p-4 shadow-xl md:bottom-4"
  >
    <div class="flex items-start gap-3">
      <span class="text-2xl">🏠</span>
      <div class="min-w-0 flex-1">
        <p class="text-sm font-semibold">Als App installieren</p>
        {#if deferred}
          <p class="mt-0.5 text-xs text-muted-foreground">
            Öffnet ohne Browserleiste und startet schneller.
          </p>
        {:else}
          <p class="mt-0.5 text-xs text-muted-foreground">
            In Safari auf <Share class="inline h-3 w-3" /> tippen, dann
            „Zum Home-Bildschirm".
          </p>
        {/if}
      </div>
      <button class="touch-target shrink-0 text-muted-foreground" onclick={dismiss} aria-label="Schließen">
        <X class="h-4 w-4" />
      </button>
    </div>

    {#if deferred}
      <div class="mt-3 flex gap-2">
        <button class="btn-outline flex-1 text-sm" onclick={dismiss}>Später</button>
        <button class="btn-primary flex-1 text-sm" onclick={install}>
          <Download class="h-4 w-4" /> Installieren
        </button>
      </div>
    {/if}
  </div>
{/if}
