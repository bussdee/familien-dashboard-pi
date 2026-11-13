<script lang="ts">
  import { t } from '$lib/i18n';
  import { MailOpen } from 'lucide-svelte';
  import { postApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { portal } from '$lib/utils/portal';
  import type { Brief } from '$lib/types';

  /**
   * Post von Mama und Papa: Liegt ungeöffnete Post bereit, erscheint beim
   * Öffnen der App ein Umschlag. Erst ein Tipp darauf zeigt, was drinsteht —
   * das Aufmachen ist der halbe Spaß. Mehrere Briefe kommen nacheinander.
   */
  let ungeoeffnet = $state<Brief[]>([]);
  let offen = $state<Brief | null>(null);
  let arbeitet = $state(false);
  /** Der Bonus des gerade gelesenen Briefs: gefeiert wird beim Schließen. */
  let bonusDanach: { punkte: number; von: string } | null = null;

  const aktuell = $derived(ungeoeffnet[0] ?? null);

  async function laden() {
    try {
      const { inbox } = await postApi.liste();
      ungeoeffnet = inbox.filter((b) => !b.opened_at);
    } catch {
      /* Beim nächsten Öffnen. */
    }
  }

  async function aufmachen() {
    if (!aktuell || arbeitet) return;
    arbeitet = true;
    try {
      const { mail, first_open } = await postApi.oeffnen(aktuell.id);
      offen = mail;
      ungeoeffnet = ungeoeffnet.slice(1);
      // Die Punkte feiern, wenn der Brief gelesen und zu ist — nicht über ihm.
      bonusDanach = first_open && mail.bonus > 0 ? { punkte: mail.bonus, von: mail.from_name } : null;
    } catch {
      ungeoeffnet = ungeoeffnet.slice(1);
    } finally {
      arbeitet = false;
    }
  }

  function schliessen() {
    offen = null;
    const uid = $session.user?.id;
    if (bonusDanach && uid) void board.award(uid, bonusDanach.punkte, t('💌 Ein Bonus von {0}', [bonusDanach.von]));
    bonusDanach = null;
  }

  $effect(() => {
    if ($session.user) void laden();
    else ungeoeffnet = [];
  });

  // Ein Handy bleibt oft offen: Beim Zurückholen in den Vordergrund nachsehen.
  $effect(() => {
    if (!$session.user) return;
    const wieder = () => document.visibilityState === 'visible' && void laden();
    document.addEventListener('visibilitychange', wieder);
    return () => document.removeEventListener('visibilitychange', wieder);
  });
</script>

{#if offen || (aktuell && !board.celebration)}
  <div use:portal class="fixed inset-0 z-[65] flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm" role="presentation">
    <div class="w-full max-w-sm animate-pop rounded-3xl border border-border bg-card p-6 text-center shadow-2xl" role="dialog" aria-modal="true" aria-label={t('Post')}>
      {#if offen}
        <p class="text-5xl" aria-hidden="true">💌</p>
        <p class="mt-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
          {t('Von {0}', [`${offen.from_emoji} ${offen.from_name}`])}
        </p>
        <p class="mt-4 whitespace-pre-line font-display text-2xl leading-snug">{offen.text}</p>
        {#if offen.bonus > 0}
          <p class="mt-4 inline-flex items-center gap-1.5 rounded-full bg-amber-500/15 px-3 py-1 text-sm font-semibold text-amber-600 dark:text-amber-400">
            🎁 {t('+{0} Punkte', [offen.bonus])}
          </p>
        {/if}
        <button class="btn-primary mt-6 w-full" onclick={schliessen}>
          {ungeoeffnet.length > 0 ? t('Noch ein Brief!') : t('Danke!')}
        </button>
      {:else if aktuell}
        <button class="group block w-full" onclick={aufmachen} disabled={arbeitet} aria-label={t('Brief öffnen')}>
          <span class="block text-7xl transition-transform group-hover:scale-110 group-active:scale-95" aria-hidden="true">✉️</span>
          <span class="mt-3 block text-lg font-semibold">{t('Du hast Post von {0}!', [aktuell.from_name])}</span>
          <span class="mt-1 block text-sm text-muted-foreground">{t('Tippe auf den Umschlag')}</span>
        </button>
        <button class="btn-primary mt-5 w-full" onclick={aufmachen} disabled={arbeitet}>
          <MailOpen class="h-4 w-4" /> {t('Aufmachen')}
        </button>
      {/if}
    </div>
  </div>
{/if}
