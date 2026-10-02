<script lang="ts">
  import { t } from '$lib/i18n';
  import { LockKeyhole, X } from 'lucide-svelte';
  import { ApiError, authApi } from '$lib/api';
  import { eltern } from '$lib/stores/eltern.svelte';
  import type { User } from '$lib/types';

  /**
   * Der Dialog für die Eltern-PIN am Wandgerät. Gesicht antippen, PIN
   * eingeben — bei der vierten Ziffer wird geprüft, ohne extra Knopf.
   */
  let eltern_ = $state<User[]>([]);
  let gewaehlt = $state<User | null>(null);
  let pin = $state('');
  let fehler = $state('');
  let prueft = $state(false);
  let feld = $state<HTMLInputElement | null>(null);

  $effect(() => {
    if (!eltern.frage) return;
    pin = '';
    fehler = '';
    authApi
      .roster()
      .then((alle) => {
        eltern_ = alle.filter((u) => u.role === 'admin');
        // Gibt es nur ein Elternteil, ist die Wahl schon getroffen.
        gewaehlt = eltern_.length === 1 ? eltern_[0] : null;
      })
      .catch(() => (fehler = t('Die Familie konnte nicht geladen werden')));
  });

  $effect(() => {
    if (gewaehlt && feld) feld.focus();
  });

  async function pruefen() {
    if (!gewaehlt || pin.length !== 4 || prueft) return;
    prueft = true;
    fehler = '';
    try {
      const r = await authApi.elternPruefen(gewaehlt.id, pin);
      eltern.freigeben(r.id, r.name || gewaehlt.name, pin);
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : t('Prüfen fehlgeschlagen');
      pin = '';
    } finally {
      prueft = false;
    }
  }

  function eingabe() {
    pin = pin.replace(/\D/g, '').slice(0, 4);
    if (pin.length === 4) void pruefen();
  }
</script>

<svelte:window onkeydown={(e) => eltern.frage && e.key === 'Escape' && eltern.abbrechen()} />

{#if eltern.frage}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-[80] flex items-end justify-center bg-black/55 backdrop-blur-sm sm:items-center sm:p-4"
    onclick={(e) => e.target === e.currentTarget && eltern.abbrechen()}
    role="presentation"
  >
    <div
      class="safe-bottom w-full max-w-sm animate-slide-up rounded-t-2xl border border-border bg-card p-5 shadow-xl sm:rounded-2xl"
      role="dialog"
      aria-modal="true"
      aria-label={t('Eltern-PIN')}
    >
      <div class="mb-4 flex items-start justify-between gap-3">
        <div>
          <h2 class="flex items-center gap-2 text-lg font-semibold">
            <LockKeyhole class="h-5 w-5" /> {t('Eltern-PIN')}
          </h2>
          <p class="text-sm text-muted-foreground">{eltern.frage}</p>
        </div>
        <button class="touch-target text-muted-foreground" onclick={() => eltern.abbrechen()} aria-label={t('Abbrechen')}>
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="mb-4 flex justify-center gap-3">
        {#each eltern_ as u (u.id)}
          <button
            class="flex w-24 flex-col items-center gap-1 rounded-2xl border p-3 transition-colors
              {gewaehlt?.id === u.id ? 'border-primary bg-primary/10' : 'border-[color:var(--haarlinie-stark)] hover:bg-accent'}"
            onclick={() => {
              gewaehlt = u;
              pin = '';
              fehler = '';
            }}
            aria-pressed={gewaehlt?.id === u.id}
          >
            <span class="flex h-12 w-12 items-center justify-center rounded-full text-3xl" style="background-color: {u.color}22">
              {u.avatar_emoji}
            </span>
            <span class="truncate text-sm font-medium">{u.name}</span>
          </button>
        {/each}
      </div>

      {#if gewaehlt}
        <input
          bind:this={feld}
          class="input text-center text-2xl tracking-[0.6em]"
          type="password"
          inputmode="numeric"
          pattern="[0-9]*"
          maxlength="4"
          autocomplete="off"
          placeholder="••••"
          aria-label={t('PIN von {0}', [gewaehlt.name])}
          bind:value={pin}
          oninput={eingabe}
          disabled={prueft}
        />
      {/if}

      {#if fehler}
        <p class="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{fehler}</p>
      {/if}

      <p class="mt-4 text-center text-xs text-muted-foreground">
        {t('Gilt zwei Minuten an diesem Tablet. Niemand wird dabei angemeldet.')}
      </p>
    </div>
  </div>
{/if}
