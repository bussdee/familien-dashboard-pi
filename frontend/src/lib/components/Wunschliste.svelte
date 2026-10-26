<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { Check, Gift, HandHeart, Hourglass, Pencil, Plus, Sparkles, X } from 'lucide-svelte';
  import { ApiError, wishesApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from './Modal.svelte';
  import type { Wish, WishOffer } from '$lib/types';

  /**
   * Die Wunschbörse.
   *
   * Die Belohnungen legen die Eltern fest. Oft will man aber etwas, das auf
   * keiner Liste steht — ins Kino gehen, ein Frühstück ans Bett, bei der
   * Freundin übernachten. Hier trägt jede Person einen Wunsch ein, auch die
   * Eltern, und jede andere kann ihn erfüllen, indem sie ein Angebot in
   * Punkten macht. Nimmt der Wünschende es an und bestätigt später, dass es
   * geklappt hat, zahlt er mit seinen Punkten an den, der erfüllt hat.
   *
   * Daneben gibt es den Weg von früher: Die Eltern legen für einen Wunsch
   * einen Preis fest, das Kind löst ihn ein, ein Elternteil bestätigt.
   *
   * Offene Wünsche sehen alle Personen; Angebote nur der Wünschende, der
   * Anbieter und die Eltern. Am Wandgerät gibt es die Liste gar nicht.
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
  const ich = $derived($session.user?.id ?? 0);

  const symbole = ['⭐', '🎬', '🍦', '🎮', '🧸', '⚽', '🎨', '🛝', '📚', '🍕', '🏊', '🎢', '🚲', '👟', '🎁', '🌙'];
  let titel = $state('');
  let symbol = $state('⭐');
  /** Für wen ein Elternteil einträgt; 0 = für sich selbst. */
  let fuer = $state(0);
  let sendet = $state(false);

  // Die Gesichter für „Für wen": alle in der Rangliste, so wie bei den Punkten.
  const personen = $derived([...board.scores].sort((a, b) => a.id - b.id));

  // Was die Eltern bepreisen können: offene Wünsche anderer (der eigene
  // Wunsch wird nicht von einem selbst bepreist).
  const offene = $derived(wuensche.filter((w) => w.status === 'offen' && !w.mine));
  const aktiv = (o: WishOffer) => ['offen', 'angenommen', 'gemeldet'].includes(o.status);
  const meinAngebot = (w: Wish) => w.offers.find((o) => o.offerer_id === ich && aktiv(o)) ?? null;
  const angenommen = (w: Wish) => w.offers.find((o) => o.status === 'angenommen' || o.status === 'gemeldet') ?? null;

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
        admin
          ? fuer ? t('Eingetragen — jetzt einen Preis festlegen.') : t('Eingetragen! Jetzt kann es jemand erfüllen.')
          : t('Eingetragen! Die anderen können es jetzt erfüllen, oder die Eltern legen einen Preis fest.'),
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

  // ---- Die Börse: erfüllen, annehmen, bezahlen ----
  let angebotFuer = $state<Wish | null>(null);
  let angebotOffen = $state(false);
  let angebotPunkte = $state('');
  let angebotNotiz = $state('');
  let angebotSendet = $state(false);
  const ANGEBOTE = [0, 10, 20, 50, 100];
  const angebotZahl = $derived(Math.max(0, Number.parseInt(angebotPunkte.replace(/\D/g, ''), 10) || 0));

  function angebotOeffnen(w: Wish) {
    const bisher = meinAngebot(w);
    angebotFuer = w;
    angebotPunkte = bisher ? String(bisher.points) : '';
    angebotNotiz = bisher?.note ?? '';
    angebotOffen = true;
  }

  async function angebotSenden(event: SubmitEvent) {
    event.preventDefault();
    if (!angebotFuer || angebotSendet || angebotPunkte.trim() === '') return;
    angebotSendet = true;
    try {
      await wishesApi.offer(angebotFuer.id, { points: angebotZahl, note: angebotNotiz.trim() });
      angebotOffen = false;
      toast(
        angebotZahl === 0
          ? t('Angebot gemacht: als Geschenk')
          : t('Angebot gemacht: {0} Punkte', [angebotZahl]),
        { ton: 'erfolg' },
      );
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte das Angebot nicht senden'), { ton: 'fehler' });
    } finally {
      angebotSendet = false;
    }
  }

  async function aktion(w: Wish, o: WishOffer, art: 'accept' | 'decline' | 'withdraw' | 'done' | 'confirm') {
    if (art === 'confirm') {
      const ok = await confirmAction({
        title: t('Hat {0} „{1}“ erfüllt?', [o.offerer_name, w.title]),
        message: o.points === 0
          ? t('Es kostet dich nichts: {0} hat es dir geschenkt.', [o.offerer_name])
          : t('Du zahlst {0} Punkte von deinem Guthaben an {1}. Level und Rangliste bleiben, wie sie sind.', [o.points, o.offerer_name]),
        confirmLabel: o.points === 0 ? t('Ja, danke!') : t('Ja, bezahlen'),
        danger: false,
      });
      if (!ok) return;
    }
    arbeitet = w.id;
    try {
      if (art === 'accept') await wishesApi.accept(w.id, o.id);
      else if (art === 'decline') await wishesApi.decline(w.id, o.id);
      else if (art === 'withdraw') await wishesApi.withdrawOffer(w.id, o.id);
      else if (art === 'done') await wishesApi.done(w.id, o.id);
      else await wishesApi.confirm(w.id, o.id);
      if (art === 'confirm') toast(t('Bezahlt. Danke, {0}!', [o.offerer_name]), { ton: 'erfolg' });
      if (art === 'accept') toast(t('Angenommen. Die Punkte sind zurückgelegt.'), { ton: 'erfolg' });
      await geaendert();
      void board.refresh().catch(() => {});
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Das hat nicht geklappt'), { ton: 'fehler' });
    } finally {
      arbeitet = null;
    }
  }

  onMount(() => void laden());
</script>

{#if person}
  <section id="wuensche" class="mt-10 scroll-mt-24">
    <header class="mb-3">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <Sparkles class="h-5 w-5 text-[color:var(--ton-belohnung)]" /> {t('Wunschbörse')}
        {#if admin && offene.length > 0}
          <span class="rounded-full bg-primary px-2 py-0.5 text-xs text-primary-foreground" title={t('Warten auf einen Preis')}>
            {offene.length}
          </span>
        {/if}
      </h2>
      <p class="mt-0.5 text-sm text-muted-foreground">
        {t('Wünsch dir etwas — und erfülle anderen einen Wunsch. Wer erfüllt, bekommt dafür Punkte vom Wünschenden.')}
        {#if guthaben !== null}
          <span class="whitespace-nowrap font-medium text-foreground">{t('Dein Guthaben: {0}', [guthaben])}</span>
        {/if}
      </p>
    </header>

    {#if fehler}
      <p class="mb-3 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
    {/if}

    <form class="card mb-4 space-y-3 p-3 sm:p-4" onsubmit={eintragen}>
      <div class="flex flex-wrap gap-2">
        <input
          class="input min-w-0 flex-1 basis-56"
          placeholder={t('Ich wünsche mir … z. B. ein Frühstück ans Bett')}
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
          {t('Sobald sich jemand etwas wünscht, steht es hier. Dein erster Wunsch gehört auch hierher.')}
        </p>
      </div>
    {:else}
      <ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {#each wuensche as w (w.id)}
          {@const reicht = guthaben !== null && w.cost !== null && guthaben >= w.cost}
          {@const anteil = guthaben !== null && w.cost ? Math.min(100, Math.round((Math.max(guthaben, 0) / w.cost) * 100)) : 0}
          {@const mein = meinAngebot(w)}
          {@const vergeben = angenommen(w)}
          <li class="card flex flex-col gap-3 p-4 {w.status === 'erfuellt' ? 'opacity-70' : ''}">
            <div class="flex items-start gap-3">
              <span class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-muted text-2xl">{w.emoji}</span>
              <div class="min-w-0 flex-1">
                <p class="font-semibold leading-snug">{w.title}</p>
                <p class="mt-0.5 text-xs text-muted-foreground">
                  {#if w.mine}{t('von dir')}{:else}{w.user_emoji} {w.user_name}{/if} ·
                  {#if w.status === 'offen'}
                    {#if w.mine}
                      {w.offer_count === 0 ? t('wartet auf Angebote') : w.offer_count === 1 ? t('1 Angebot') : t('{0} Angebote', [w.offer_count])}
                    {:else}
                      {t('wartet auf jemanden, der es erfüllt')}
                    {/if}
                  {:else if w.status === 'vergeben'}
                    {w.fulfilled_by === ich ? t('du erfüllst das') : t('{0} erfüllt das', [w.fulfilled_by_name ?? ''])}
                  {:else if w.status === 'bepreist'}
                    <span class="text-sm font-semibold tabular-nums text-[color:var(--ton-belohnung)]">{t('{0} Punkte', [w.cost])}</span>
                  {:else if w.status === 'angefragt'}
                    {t('angefragt — wartet auf die Eltern')}
                  {:else if w.status === 'erfuellt'}
                    {w.fulfilled_by_name ? t('erfüllt von {0} 🎉', [w.fulfilled_by_name]) : t('erfüllt 🎉')}
                  {:else}
                    {t('leider nein')}
                  {/if}
                </p>
              </div>
              {#if admin && !w.mine}
                <button
                  class="touch-target -mr-2 -mt-2 text-muted-foreground"
                  onclick={() => oeffnen(w)}
                  aria-label={t('„{0}“ bearbeiten', [w.title])}
                >
                  <Pencil class="h-4 w-4" />
                </button>
              {:else if w.mine && w.status !== 'angefragt' && w.status !== 'vergeben'}
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

            <!-- Die Angebote auf meinen eigenen Wunsch: annehmen oder ablehnen. -->
            {#if w.mine && w.status === 'offen' && w.offers.length > 0}
              <ul class="space-y-2">
                {#each w.offers.filter((o) => o.status === 'offen') as o (o.id)}
                  <li class="rounded-xl bg-muted/50 p-3">
                    <p class="flex items-center gap-2 text-sm">
                      <span class="text-lg leading-none">{o.offerer_emoji}</span>
                      <span class="min-w-0 flex-1 truncate font-medium">{o.offerer_name}</span>
                      <span class="shrink-0 font-semibold tabular-nums text-[color:var(--ton-belohnung)]">
                        {o.points === 0 ? t('geschenkt') : t('{0} Punkte', [o.points])}
                      </span>
                    </p>
                    {#if o.note}<p class="mt-1 text-xs text-muted-foreground">💬 {o.note}</p>{/if}
                    <div class="mt-2 flex gap-2">
                      <button
                        class="btn-primary flex-1 !min-h-[40px] text-sm"
                        onclick={() => aktion(w, o, 'accept')}
                        disabled={arbeitet !== null || (guthaben !== null && guthaben < o.points)}
                        title={guthaben !== null && guthaben < o.points ? t('Dafür fehlen noch {0} Punkte', [o.points - guthaben]) : ''}
                      >
                        {t('Annehmen')}
                      </button>
                      <button class="btn-outline !min-h-[40px] px-3 text-sm" onclick={() => aktion(w, o, 'decline')} disabled={arbeitet !== null}>
                        {t('Ablehnen')}
                      </button>
                    </div>
                    {#if guthaben !== null && guthaben < o.points}
                      <p class="mt-1.5 text-xs text-amber-700 dark:text-amber-400">{t('Dafür fehlen noch {0} Punkte', [o.points - guthaben])}</p>
                    {/if}
                  </li>
                {/each}
              </ul>
            {/if}

            <!-- Ein angenommenes Angebot, von beiden Seiten gesehen. -->
            {#if w.status === 'vergeben' && vergeben}
              <div class="rounded-xl bg-primary/10 p-3 text-sm">
                <p class="flex items-center gap-2">
                  <HandHeart class="h-4 w-4 shrink-0 text-primary" />
                  <span class="min-w-0 flex-1">
                    {#if w.mine}
                      {t('{0} erfüllt das für {1} Punkte.', [vergeben.offerer_name, vergeben.points])}
                    {:else}
                      {t('Du erfüllst das für {0} Punkte.', [vergeben.points])}
                    {/if}
                  </span>
                </p>
                {#if vergeben.note}<p class="mt-1 text-xs text-muted-foreground">💬 {vergeben.note}</p>{/if}
                {#if vergeben.status === 'gemeldet'}
                  <p class="mt-1.5 flex items-center gap-1.5 text-xs font-medium text-primary">
                    <Check class="h-3.5 w-3.5" /> {w.mine ? t('Gemeldet: erledigt') : t('Du hast gemeldet: erledigt')}
                  </p>
                {/if}
                <div class="mt-2 flex flex-wrap gap-2">
                  {#if w.mine}
                    <button class="btn-primary flex-1 !min-h-[40px] text-sm" onclick={() => aktion(w, vergeben, 'confirm')} disabled={arbeitet !== null}>
                      {vergeben.points === 0 ? t('Ist erledigt') : t('Ist erledigt — bezahlen')}
                    </button>
                    <button class="btn-outline !min-h-[40px] px-3 text-sm" onclick={() => aktion(w, vergeben, 'decline')} disabled={arbeitet !== null}>
                      {t('Doch nicht')}
                    </button>
                  {:else if vergeben.offerer_id === ich}
                    {#if vergeben.status === 'angenommen'}
                      <button class="btn-primary flex-1 !min-h-[40px] text-sm" onclick={() => aktion(w, vergeben, 'done')} disabled={arbeitet !== null}>
                        {t('Ich bin fertig')}
                      </button>
                    {/if}
                    <button class="btn-outline !min-h-[40px] px-3 text-sm" onclick={() => aktion(w, vergeben, 'withdraw')} disabled={arbeitet !== null}>
                      {t('Zurückziehen')}
                    </button>
                  {/if}
                </div>
              </div>
            {/if}

            <!-- Der Wunsch eines anderen: erfüllen. -->
            {#if !w.mine && w.status === 'offen' && person}
              {#if mein}
                <div class="rounded-xl bg-muted/50 p-3 text-sm">
                  <p>
                    {t('Dein Angebot:')}
                    <span class="font-semibold tabular-nums">{mein.points === 0 ? t('geschenkt') : t('{0} Punkte', [mein.points])}</span>
                  </p>
                  <div class="mt-2 flex gap-2">
                    <button class="btn-outline flex-1 !min-h-[40px] text-sm" onclick={() => angebotOeffnen(w)}>{t('Ändern')}</button>
                    <button class="btn-outline !min-h-[40px] px-3 text-sm" onclick={() => aktion(w, mein, 'withdraw')} disabled={arbeitet !== null}>{t('Zurückziehen')}</button>
                  </div>
                </div>
              {:else}
                <button class="btn-primary w-full" onclick={() => angebotOeffnen(w)}>
                  <HandHeart class="h-4 w-4" /> {t('Das erfülle ich')}
                </button>
              {/if}
            {/if}

            <!-- Der Weg von früher: Eltern legen einen Preis fest. -->
            {#if admin && !w.mine && w.status === 'offen'}
              <button class="btn-outline mt-auto w-full text-sm" onclick={() => oeffnen(w)}>
                {t('Familien-Preis festlegen')}
              </button>
            {:else if w.mine && w.status === 'bepreist' && w.cost !== null && guthaben !== null}
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


  <Modal bind:open={angebotOffen} title={t('Das erfülle ich: {0}', [angebotFuer?.title ?? ''])}>
    {#if angebotFuer}
      <form class="space-y-3" onsubmit={angebotSenden}>
        <p class="text-sm text-muted-foreground">
          {t('{0} zahlt dir die Punkte, wenn der Wunsch erfüllt ist. Du bestimmst den Preis — 0 heißt: geschenkt.', [angebotFuer.user_name])}
        </p>
        <label class="block text-xs text-muted-foreground">
          {t('Mein Preis in Punkten')}
          <input class="input mt-1" inputmode="numeric" bind:value={angebotPunkte} maxlength="6" placeholder={t('z. B. 20')} />
        </label>
        <div class="flex flex-wrap gap-1.5">
          {#each ANGEBOTE as p (p)}
            <button type="button" class="chip" aria-pressed={angebotPunkte !== '' && angebotZahl === p} onclick={() => (angebotPunkte = String(p))}>
              {p === 0 ? t('Geschenkt') : p}
            </button>
          {/each}
        </div>
        <label class="block text-xs text-muted-foreground">
          {t('Hinweis (optional)')}
          <input class="input mt-1" bind:value={angebotNotiz} maxlength="200" placeholder={t('z. B. am Samstag')} />
        </label>
        <button class="btn-primary w-full" disabled={angebotPunkte.trim() === '' || angebotSendet}>
          {angebotSendet ? t('Einen Moment…') : t('Angebot machen')}
        </button>
      </form>
    {/if}
  </Modal>

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
