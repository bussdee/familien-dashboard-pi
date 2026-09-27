<script lang="ts">
  import { onMount } from 'svelte';
  import { formatDistanceToNow, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { Check, Gift, Pencil, Plus, Sparkles, Undo2, UserRound, X } from 'lucide-svelte';
  import { ApiError, rewardsApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import type { Redemption, Reward, RewardOverview } from '$lib/types';

  /**
   * Wofür die Punkte da sind.
   *
   * Eine Zahl, die nur steigt, trägt bei einem Kind etwa drei Wochen. Danach
   * fragt es: „Und was krieg ich dafür?" Hier steht die Antwort — festgelegt
   * von den Eltern, eingelöst mit einem Tipp, bestätigt von einem Elternteil.
   *
   * Level und Rangliste bleiben davon unberührt: Wer sich ein Eis holt,
   * fällt nicht auf Platz drei zurück. Ausgegeben wird das Guthaben.
   */
  let daten = $state<RewardOverview | null>(null);
  let fehler = $state('');
  let arbeitet = $state<number | null>(null);

  const admin = $derived($session.user?.role === 'admin');
  const geraet = $derived($session.device);
  const guthaben = $derived(daten?.balance ?? null);
  const offene = $derived((daten?.redemptions ?? []).filter((r) => r.status === 'offen'));
  const erledigte = $derived((daten?.redemptions ?? []).filter((r) => r.status !== 'offen'));

  async function laden() {
    try {
      daten = await rewardsApi.overview();
      fehler = '';
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Die Belohnungen konnten nicht geladen werden.';
    }
  }

  async function einloesen(r: Reward) {
    const ok = await confirmAction({
      title: `${r.emoji} ${r.title}?`,
      message: `Kostet ${r.cost} Punkte aus deinem Guthaben. Ein Elternteil bestätigt, wenn es so weit ist. Level und Rangliste bleiben, wie sie sind.`,
      confirmLabel: 'Einlösen',
      danger: false,
    });
    if (!ok) return;
    arbeitet = r.id;
    try {
      const res = await rewardsApi.redeem(r.id);
      toast(`Angefragt! Noch ${res.balance} Punkte übrig.`, { ton: 'erfolg' });
      await Promise.all([laden(), board.refresh().catch(() => {})]);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht einlösen', { ton: 'fehler' });
    } finally {
      arbeitet = null;
    }
  }

  async function zurueckziehen(rd: Redemption) {
    try {
      await rewardsApi.cancel(rd.id);
      toast('Zurückgezogen — die Punkte sind wieder da.');
      await Promise.all([laden(), board.refresh().catch(() => {})]);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht zurückziehen', { ton: 'fehler' });
    }
  }

  async function entscheiden(rd: Redemption, status: 'eingeloest' | 'abgelehnt') {
    try {
      await rewardsApi.decide(rd.id, status);
      toast(
        status === 'eingeloest'
          ? `${rd.emoji} ${rd.title} für ${rd.user_name} eingelöst`
          : `Abgelehnt — ${rd.user_name} bekommt die ${rd.cost} Punkte zurück`,
        { ton: status === 'eingeloest' ? 'erfolg' : 'info' },
      );
      await Promise.all([laden(), board.refresh().catch(() => {})]);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht speichern', { ton: 'fehler' });
    }
  }

  // ---- Verwalten (Eltern) ----
  const symbole = ['🎁', '📱', '🍦', '🎬', '🌙', '🍕', '🎮', '🧸', '🎨', '⚽', '🏊', '🍫', '📚', '🎢', '💶', '🛝'];
  let formOffen = $state(false);
  let bearbeitetId = $state<number | null>(null);
  let entwurf = $state({ title: '', emoji: '🎁', cost: 50, active: true });

  function neu() {
    bearbeitetId = null;
    entwurf = { title: '', emoji: '🎁', cost: 50, active: true };
    formOffen = true;
  }

  function bearbeiten(r: Reward) {
    bearbeitetId = r.id;
    entwurf = { title: r.title, emoji: r.emoji, cost: r.cost, active: r.active };
    formOffen = true;
  }

  async function speichern(event: SubmitEvent) {
    event.preventDefault();
    try {
      if (bearbeitetId !== null) await rewardsApi.update(bearbeitetId, entwurf);
      else await rewardsApi.create(entwurf);
      formOffen = false;
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht speichern', { ton: 'fehler' });
    }
  }

  async function entfernen() {
    if (bearbeitetId === null) return;
    const ok = await confirmAction({
      title: `„${entwurf.title}" löschen?`,
      message: 'Verschwindet aus dem Angebot. Bereits eingelöste bleiben in der Liste stehen.',
    });
    if (!ok) return;
    try {
      await rewardsApi.remove(bearbeitetId);
      formOffen = false;
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht löschen', { ton: 'fehler' });
    }
  }

  const wann = (iso: string) => formatDistanceToNow(parseISO(iso), { locale: de, addSuffix: true });

  onMount(() => void laden());
</script>

<svelte:head><title>Belohnungen · Familien Dashboard</title></svelte:head>

<div class="mx-auto w-full max-w-4xl px-4 py-5 sm:py-7">
  <header class="mb-6 flex flex-wrap items-end justify-between gap-4">
    <div>
      <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
        Punkte eintauschen
      </p>
      <h1 class="seiten-titel mt-1">Belohnungen</h1>
    </div>
    {#if admin}
      <button class="btn-primary" onclick={neu}>
        <Plus class="h-4 w-4" /> Belohnung
      </button>
    {/if}
  </header>

  {#if guthaben !== null}
    <!-- Das Guthaben als grosse Zahl: Darum geht es auf dieser Seite. -->
    <section
      class="mb-6 flex items-center gap-5 rounded-3xl border border-[color:var(--haarlinie-stark)] bg-gradient-to-br from-primary/15 via-card/60 to-card/40 p-5 sm:p-6"
    >
      <span class="kachel-symbol !h-14 !w-14 !rounded-2xl" style="--ton: var(--ton-belohnung)">
        <Sparkles class="h-7 w-7" />
      </span>
      <div class="min-w-0">
        <p class="text-sm text-muted-foreground">Dein Guthaben</p>
        <p class="font-display text-5xl font-medium leading-none tracking-tight">
          {guthaben}<span class="ml-1.5 text-lg font-light text-muted-foreground">Punkte</span>
        </p>
        <p class="mt-1.5 text-xs text-muted-foreground">
          Einlösen kostet Guthaben, keine Level. Die Rangliste bleibt, wie sie ist.
        </p>
      </div>
    </section>
  {:else if geraet}
    <a
      href="/login"
      class="mb-6 flex items-center gap-3 rounded-2xl bg-muted/50 p-4 text-sm transition-colors hover:bg-accent"
    >
      <UserRound class="h-5 w-5 shrink-0" />
      <span>Zum Einlösen bitte anmelden — am Wandgerät könnte sonst jeder das Guthaben der anderen ausgeben.</span>
    </a>
  {/if}

  {#if fehler}
    <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
  {/if}

  {#if admin && offene.length > 0}
    <section class="card mb-6 p-4 sm:p-5">
      <h2 class="mb-3 flex items-center gap-2 font-semibold">
        <Gift class="h-5 w-5" /> Warten auf dich
        <span class="rounded-full bg-primary px-2 py-0.5 text-xs text-primary-foreground">{offene.length}</span>
      </h2>
      <ul class="divide-y divide-[color:var(--haarlinie)]">
        {#each offene as rd (rd.id)}
          <li class="flex flex-wrap items-center gap-x-3 gap-y-2 py-2.5">
            <span class="text-2xl">{rd.emoji}</span>
            <div class="min-w-[10rem] flex-1">
              <p class="text-sm font-medium">{rd.user_emoji} {rd.user_name}: {rd.title}</p>
              <p class="text-xs text-muted-foreground">{rd.cost} Punkte · {wann(rd.created_at)}</p>
            </div>
            <div class="ml-auto flex gap-2">
            <button
              class="btn-outline px-3 text-muted-foreground"
              onclick={() => entscheiden(rd, 'abgelehnt')}
              aria-label="Ablehnen"
              title="Ablehnen — Punkte zurück"
            >
              <X class="h-4 w-4" />
            </button>
            <button class="btn-primary px-3" onclick={() => entscheiden(rd, 'eingeloest')}>
              <Check class="h-4 w-4" /> Eingelöst
            </button>
            </div>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  {#if daten}
    {#if daten.rewards.length === 0}
      <div class="card flex flex-col items-center gap-2 p-10 text-center">
        <Gift class="h-10 w-10 text-muted-foreground opacity-40" />
        <p class="font-medium">Noch keine Belohnungen</p>
        <p class="max-w-sm text-sm text-muted-foreground">
          {admin
            ? 'Leg fest, wofür die Punkte eingetauscht werden können — Bildschirmzeit, ein Eis, der Film am Familienabend.'
            : 'Ein Elternteil legt fest, wofür die Punkte eingetauscht werden können.'}
        </p>
      </div>
    {:else}
      <ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {#each daten.rewards as r (r.id)}
          {@const reicht = guthaben !== null && guthaben >= r.cost}
          {@const anteil = guthaben !== null ? Math.min(100, Math.round((guthaben / r.cost) * 100)) : 0}
          <li
            class="card relative flex flex-col gap-3 p-4 transition-opacity {r.active ? '' : 'opacity-50'}"
          >
            <div class="flex items-start gap-3">
              <span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-muted text-3xl">
                {r.emoji}
              </span>
              <div class="min-w-0 flex-1">
                <p class="font-semibold leading-snug">{r.title}</p>
                <p class="mt-0.5 text-sm font-semibold tabular-nums text-[color:var(--ton-belohnung)]">
                  {r.cost} Punkte
                  {#if !r.active}<span class="font-normal text-muted-foreground"> · ausgeblendet</span>{/if}
                </p>
              </div>
              {#if admin}
                <button
                  class="touch-target -mr-2 -mt-2 text-muted-foreground"
                  onclick={() => bearbeiten(r)}
                  aria-label="{r.title} bearbeiten"
                >
                  <Pencil class="h-4 w-4" />
                </button>
              {/if}
            </div>

            {#if guthaben !== null}
              {#if reicht}
                <button
                  class="btn-primary mt-auto w-full"
                  onclick={() => einloesen(r)}
                  disabled={arbeitet !== null || !r.active}
                >
                  {arbeitet === r.id ? 'Einen Moment…' : 'Einlösen'}
                </button>
              {:else}
                <div class="mt-auto">
                  <div class="mb-1 flex justify-between text-xs text-muted-foreground">
                    <span>noch {r.cost - guthaben} Punkte</span>
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
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    {#if !admin && (daten.redemptions.length > 0)}
      <section class="mt-8">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Deine Anfragen
        </h2>
        <ul class="card divide-y divide-[color:var(--haarlinie)]">
          {#each daten.redemptions as rd (rd.id)}
            <li class="flex items-center gap-3 p-3">
              <span class="text-2xl">{rd.emoji}</span>
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium">{rd.title}</p>
                <p class="text-xs text-muted-foreground">{rd.cost} Punkte · {wann(rd.created_at)}</p>
              </div>
              {#if rd.status === 'offen'}
                <span class="rounded-full bg-amber-500/15 px-2.5 py-1 text-xs font-medium text-amber-700 dark:text-amber-400">
                  wartet
                </span>
                <button
                  class="touch-target text-muted-foreground"
                  onclick={() => zurueckziehen(rd)}
                  aria-label="Anfrage zurückziehen"
                  title="Zurückziehen"
                >
                  <Undo2 class="h-4 w-4" />
                </button>
              {:else if rd.status === 'eingeloest'}
                <span class="rounded-full bg-success/15 px-2.5 py-1 text-xs font-medium text-success">eingelöst</span>
              {:else}
                <span class="rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">abgelehnt</span>
              {/if}
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if admin && erledigte.length > 0}
      <section class="mt-8">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Zuletzt entschieden
        </h2>
        <ul class="card divide-y divide-[color:var(--haarlinie)]">
          {#each erledigte as rd (rd.id)}
            <li class="flex items-center gap-3 p-3 text-sm">
              <span class="text-xl">{rd.emoji}</span>
              <span class="min-w-0 flex-1 truncate">{rd.user_emoji} {rd.user_name}: {rd.title}</span>
              <span class="shrink-0 text-xs {rd.status === 'eingeloest' ? 'text-success' : 'text-muted-foreground'}">
                {rd.status === 'eingeloest' ? 'eingelöst' : 'abgelehnt'}
              </span>
            </li>
          {/each}
        </ul>
      </section>
    {/if}
  {:else if !fehler}
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {#each Array(3) as _, i (i)}
        <div class="h-36 animate-pulse rounded-2xl bg-muted/40"></div>
      {/each}
    </div>
  {/if}
</div>

<Modal bind:open={formOffen} title={bearbeitetId !== null ? 'Belohnung bearbeiten' : 'Neue Belohnung'}>
  <form class="space-y-3" onsubmit={speichern}>
    <input class="input" placeholder="Wofür? z. B. Ein Eis" bind:value={entwurf.title} maxlength="80" />
    <div class="grid grid-cols-8 gap-1">
      {#each symbole as s (s)}
        <button
          type="button"
          class="flex h-10 items-center justify-center rounded-lg text-xl transition-colors
            {entwurf.emoji === s ? 'bg-primary/15 ring-2 ring-primary' : 'hover:bg-accent'}"
          onclick={() => (entwurf.emoji = s)}
          aria-label="Symbol {s}"
          aria-pressed={entwurf.emoji === s}
        >
          {s}
        </button>
      {/each}
    </div>
    <label class="block text-xs text-muted-foreground">
      Kostet Punkte
      <input class="input mt-1" type="number" min="1" max="100000" bind:value={entwurf.cost} />
    </label>
    <p class="text-xs text-muted-foreground">
      Zum Vergleich: Müll rausbringen bringt 10 Punkte, ein Wocheneinkauf bis zu 80.
    </p>
    <label class="flex items-center gap-2 text-sm">
      <input type="checkbox" class="h-4 w-4 rounded" bind:checked={entwurf.active} />
      Im Angebot (abwählen blendet sie aus, ohne sie zu löschen)
    </label>
    <div class="flex gap-2 pt-1">
      {#if bearbeitetId !== null}
        <button type="button" class="btn-outline px-3 text-destructive" onclick={entfernen}>Löschen</button>
      {/if}
      <button class="btn-primary flex-1" disabled={!entwurf.title.trim() || entwurf.cost < 1}>
        Speichern
      </button>
    </div>
  </form>
</Modal>
