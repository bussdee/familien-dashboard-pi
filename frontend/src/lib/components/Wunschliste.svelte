<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { Gift, Hourglass, Pencil, Plus, Sparkles, X } from 'lucide-svelte';
  import { ApiError, wishesApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from './Modal.svelte';
  import type { Wish } from '$lib/types';

  /**
   * Die Wunschliste.
   *
   * Die Belohnungen legen die Eltern fest. Ein Kind will aber oft etwas, das
   * auf keiner Liste steht — ins Kino gehen, bei der Freundin übernachten.
   * Hier trägt es den Wunsch ein, die Eltern sagen, wie viele Punkte er
   * kostet, und sobald das Guthaben reicht, löst das Kind ihn ein. Danach
   * läuft alles wie bei einer Belohnung: Ein Elternteil bestätigt.
   *
   * Ein Wunsch gehört dem Kind: Es sieht nur seine eigenen, die Eltern sehen
   * alle. Am Wandgerät gibt es die Liste gar nicht.
   */
  let {
    guthaben = null,
    /** Nach jeder Änderung: die Seite lädt Guthaben und Anfragen neu. */
    onGeaendert,
  }: { guthaben?: number | null; onGeaendert?: () => void | Promise<void> } = $props();

  let wuensche = $state<Wish[]>([]);
  let geladen = $state(false);
  let fehler = $state('');
  let arbeitet = $state<number | null>(null);

  const admin = $derived($session.user?.role === 'admin');
  const person = $derived(!!$session.user);

  const symbole = ['⭐', '🎬', '🍦', '🎮', '🧸', '⚽', '🎨', '🛝', '📚', '🍕', '🏊', '🎢', '🚲', '👟', '🎁', '🌙'];
  let titel = $state('');
  let symbol = $state('⭐');
  /** Für wen ein Elternteil einträgt; 0 = für sich selbst. */
  let fuer = $state(0);
  let sendet = $state(false);

  // Die Gesichter für „Für wen": alle in der Rangliste, so wie bei den Punkten.
  const personen = $derived([...board.scores].sort((a, b) => a.id - b.id));

  const offene = $derived(wuensche.filter((w) => w.status === 'offen'));

  async function laden() {
    try {
      wuensche = await wishesApi.list();
      fehler = '';
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : t('Die Wünsche konnten nicht geladen werden.');
    } finally {
      geladen = true;
    }
  }

  async function geaendert() {
    await laden();
    await onGeaendert?.();
  }

  async function eintragen(event: SubmitEvent) {
    event.preventDefault();
    const text = titel.trim();
    if (!text || sendet) return;
    sendet = true;
    try {
      await wishesApi.create({ title: text, emoji: symbol, ...(admin && fuer ? { user_id: fuer } : {}) });
      titel = '';
      symbol = '⭐';
      toast(
        admin ? t('Eingetragen — jetzt einen Preis festlegen.') : t('Eingetragen! Die Eltern legen jetzt fest, wie viele Punkte das kostet.'),
        { ton: 'erfolg' },
      );
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte den Wunsch nicht eintragen'), { ton: 'fehler' });
    } finally {
      sendet = false;
    }
  }

  async function einloesen(w: Wish) {
    if (w.cost === null) return;
    const ok = await confirmAction({
      title: `${w.emoji} ${w.title}?`,
      message: t('Kostet {0} Punkte aus deinem Guthaben. Ein Elternteil bestätigt, wenn es so weit ist. Level und Rangliste bleiben, wie sie sind.', [w.cost]),
      confirmLabel: t('Einlösen'),
      danger: false,
    });
    if (!ok) return;
    arbeitet = w.id;
    try {
      const res = await wishesApi.redeem(w.id);
      toast(t('Angefragt! Noch {0} Punkte übrig.', [res.balance]), { ton: 'erfolg' });
      await geaendert();
      void board.refresh().catch(() => {});
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht einlösen'), { ton: 'fehler' });
    } finally {
      arbeitet = null;
    }
  }

  async function zuruecknehmen(w: Wish) {
    try {
      await wishesApi.withdraw(w.id);
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht zurücknehmen'), { ton: 'fehler' });
    }
  }

  // ---- Eltern: Preis festlegen ----
  const PREISE = [20, 50, 100, 200];
  let offenesFenster = $state(false);
  let gewaehlt = $state<Wish | null>(null);
  let preis = $state('');
  let hinweis = $state('');
  let titelNeu = $state('');
  let speichert = $state(false);

  function oeffnen(w: Wish) {
    if (!admin) return;
    gewaehlt = w;
    preis = w.cost !== null ? String(w.cost) : '';
    hinweis = w.note;
    titelNeu = w.title;
    offenesFenster = true;
  }

  const preisZahl = $derived(Number.parseInt(preis.replace(/\D/g, ''), 10) || 0);

  async function festlegen(event: SubmitEvent) {
    event.preventDefault();
    if (!gewaehlt || speichert || preisZahl < 1) return;
    speichert = true;
    try {
      await wishesApi.decide(gewaehlt.id, { cost: preisZahl, note: hinweis, title: titelNeu });
      offenesFenster = false;
      toast(t('{0} Punkte für „{1}“ festgelegt', [preisZahl, titelNeu.trim() || gewaehlt.title]), { ton: 'erfolg' });
      await geaendert();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      speichert = false;
    }
  }

  async function ablehnen() {
    if (!gewaehlt || speichert) return;
    speichert = true;
    try {
      await wishesApi.decide(gewaehlt.id, { reject: true, note: hinweis });
      offenesFenster = false;
      await geaendert();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      speichert = false;
    }
  }

  async function loeschen() {
    if (!gewaehlt) return;
    const w = gewaehlt;
    const ok = await confirmAction({
      title: t('„{0}“ löschen?', [w.title]),
      message: t('Der Wunsch verschwindet von der Liste.'),
    });
    if (!ok) return;
    try {
      await wishesApi.remove(w.id);
      offenesFenster = false;
      await geaendert();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht löschen'), { ton: 'fehler' });
    }
  }

  onMount(() => void laden());
</script>

{#if person}
  <section id="wuensche" class="mt-10 scroll-mt-24">
    <header class="mb-3">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <Sparkles class="h-5 w-5 text-[color:var(--ton-belohnung)]" /> {t('Wunschliste')}
        {#if admin && offene.length > 0}
          <span class="rounded-full bg-primary px-2 py-0.5 text-xs text-primary-foreground" title={t('Warten auf einen Preis')}>
            {offene.length}
          </span>
        {/if}
      </h2>
      <p class="mt-0.5 text-sm text-muted-foreground">
        {admin
          ? t('Was sich die Kinder wünschen. Legt fest, wie viele Punkte es kostet — dann können sie es sich „kaufen“.')
          : t('Was wünschst du dir? Trag es ein, die Eltern legen fest, wie viele Punkte es kostet.')}
      </p>
    </header>

    {#if fehler}
      <p class="mb-3 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
    {/if}

    <form class="card mb-4 space-y-3 p-3 sm:p-4" onsubmit={eintragen}>
      <div class="flex flex-wrap gap-2">
        <input
          class="input min-w-0 flex-1 basis-56"
          placeholder={t('Ich wünsche mir … z. B. ins Kino gehen')}
          bind:value={titel}
          maxlength="80"
          aria-label={t('Dein Wunsch')}
        />
        {#if admin && personen.length > 0}
          <select class="input w-auto" bind:value={fuer} aria-label={t('Für wen')}>
            <option value={0}>{t('Für mich')}</option>
            {#each personen as p (p.id)}
              <option value={p.id}>{p.avatar_emoji} {p.name}</option>
            {/each}
          </select>
        {/if}
        <button class="btn-primary shrink-0" disabled={!titel.trim() || sendet}>
          <Plus class="h-4 w-4" /> {t('Eintragen')}
        </button>
      </div>
      <div class="flex flex-wrap gap-1" role="radiogroup" aria-label={t('Symbol')}>
        {#each symbole as s (s)}
          <button
            type="button"
            role="radio"
            aria-checked={symbol === s}
            class="flex h-9 w-9 items-center justify-center rounded-lg text-lg transition-colors
              {symbol === s ? 'bg-primary/15 ring-2 ring-primary' : 'hover:bg-accent'}"
            onclick={() => (symbol = s)}
            aria-label={t('Symbol {0}', [s])}
          >
            {s}
          </button>
        {/each}
      </div>
    </form>

    {#if geladen && wuensche.length === 0}
      <div class="card flex flex-col items-center gap-2 p-8 text-center">
        <Gift class="h-9 w-9 text-muted-foreground opacity-40" />
        <p class="font-medium">{t('Noch keine Wünsche')}</p>
        <p class="max-w-sm text-sm text-muted-foreground">
          {admin ? t('Sobald sich ein Kind etwas wünscht, steht es hier — mit einem Feld für den Preis.') : t('Dein erster Wunsch gehört hierher.')}
        </p>
      </div>
    {:else}
      <ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {#each wuensche as w (w.id)}
          {@const reicht = guthaben !== null && w.cost !== null && guthaben >= w.cost}
          {@const anteil = guthaben !== null && w.cost ? Math.min(100, Math.round((Math.max(guthaben, 0) / w.cost) * 100)) : 0}
          <li class="card flex flex-col gap-3 p-4 {w.status === 'erfuellt' ? 'opacity-70' : ''}">
            <div class="flex items-start gap-3">
              <span class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-muted text-2xl">{w.emoji}</span>
              <div class="min-w-0 flex-1">
                <p class="font-semibold leading-snug">{w.title}</p>
                <p class="mt-0.5 text-xs text-muted-foreground">
                  {#if admin}{w.user_emoji} {w.user_name} ·{/if}
                  {#if w.status === 'offen'}
                    {t('wartet auf einen Preis')}
                  {:else if w.status === 'bepreist'}
                    <span class="text-sm font-semibold tabular-nums text-[color:var(--ton-belohnung)]">{t('{0} Punkte', [w.cost])}</span>
                  {:else if w.status === 'angefragt'}
                    {t('angefragt — wartet auf die Eltern')}
                  {:else if w.status === 'erfuellt'}
                    {t('erfüllt 🎉')}
                  {:else}
                    {t('leider nein')}
                  {/if}
                </p>
              </div>
              {#if admin}
                <button
                  class="touch-target -mr-2 -mt-2 text-muted-foreground"
                  onclick={() => oeffnen(w)}
                  aria-label={t('„{0}“ bearbeiten', [w.title])}
                >
                  <Pencil class="h-4 w-4" />
                </button>
              {:else if w.status !== 'angefragt'}
                <button
                  class="touch-target -mr-2 -mt-2 text-muted-foreground"
                  onclick={() => zuruecknehmen(w)}
                  aria-label={t('„{0}“ zurücknehmen', [w.title])}
                  title={t('Zurücknehmen')}
                >
                  <X class="h-4 w-4" />
                </button>
              {/if}
            </div>

            {#if w.note && (w.status === 'abgelehnt' || w.status === 'bepreist')}
              <p class="rounded-lg bg-muted/50 px-3 py-2 text-xs text-muted-foreground">💬 {w.note}</p>
            {/if}

            {#if admin && w.status === 'offen'}
              <button class="btn-primary mt-auto w-full" onclick={() => oeffnen(w)}>
                {t('Preis festlegen')}
              </button>
            {:else if !admin && w.status === 'bepreist' && w.cost !== null && guthaben !== null}
              {#if reicht}
                <button class="btn-primary mt-auto w-full" onclick={() => einloesen(w)} disabled={arbeitet !== null}>
                  {arbeitet === w.id ? t('Einen Moment…') : t('Einlösen')}
                </button>
              {:else}
                <div class="mt-auto">
                  <div class="mb-1 flex justify-between text-xs text-muted-foreground">
                    <span>{t('noch {0} Punkte', [w.cost - Math.max(guthaben, 0)])}</span>
                    <span class="tabular-nums">{anteil} %</span>
                  </div>
                  <div class="h-2 overflow-hidden rounded-full bg-muted">
                    <div
                      class="h-full rounded-full bg-gradient-to-r from-primary to-[color:var(--ton-belohnung)] transition-[width] duration-700"
                      style="width: {anteil}%"
                    ></div>
                  </div>
                </div>
              {/if}
            {:else if w.status === 'angefragt'}
              <p class="mt-auto flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400">
                <Hourglass class="h-3.5 w-3.5" /> {t('Die Punkte sind schon reserviert.')}
              </p>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  {#if admin}
    <Modal bind:open={offenesFenster} title={t('Wunsch von {0}', [gewaehlt?.user_name ?? ''])}>
      {#if gewaehlt}
        <form class="space-y-3" onsubmit={festlegen}>
          {#if gewaehlt.status === 'angefragt' || gewaehlt.status === 'erfuellt'}
            <p class="text-sm text-muted-foreground">
              {t('„{0}“ ist schon angefragt oder erfüllt — das entscheidest du oben bei den Belohnungen.', [gewaehlt.title])}
            </p>
            <button type="button" class="btn-outline w-full text-destructive" onclick={loeschen}>{t('Löschen')}</button>
          {:else}
            <label class="block text-xs text-muted-foreground">
              {t('Wunsch')}
              <input class="input mt-1" bind:value={titelNeu} maxlength="80" />
            </label>

            <label class="block text-xs text-muted-foreground">
              {t('Kostet Punkte')}
              <input
                class="input mt-1"
                inputmode="numeric"
                bind:value={preis}
                placeholder={t('z. B. 50')}
                maxlength="6"
              />
            </label>
            <div class="flex flex-wrap gap-1.5">
              {#each PREISE as p (p)}
                <button type="button" class="chip" aria-pressed={preisZahl === p} onclick={() => (preis = String(p))}>
                  {p}
                </button>
              {/each}
            </div>
            <p class="text-xs text-muted-foreground">
              {t('Zum Vergleich: Müll rausbringen bringt 10 Punkte, ein Wocheneinkauf bis zu 80.')}
            </p>

            <label class="block text-xs text-muted-foreground">
              {t('Hinweis für das Kind (optional)')}
              <input class="input mt-1" bind:value={hinweis} maxlength="200" placeholder={t('z. B. am Wochenende')} />
            </label>

            <button class="btn-primary w-full" disabled={preisZahl < 1 || speichert}>
              {t('Preis festlegen')}
            </button>
            <div class="flex gap-2">
              <button type="button" class="btn-outline flex-1" onclick={ablehnen} disabled={speichert}>
                {t('Leider nein')}
              </button>
              <button type="button" class="btn-outline px-3 text-destructive" onclick={loeschen} aria-label={t('Löschen')}>
                {t('Löschen')}
              </button>
            </div>
          {/if}
        </form>
      {/if}
    </Modal>
  {/if}
{/if}
