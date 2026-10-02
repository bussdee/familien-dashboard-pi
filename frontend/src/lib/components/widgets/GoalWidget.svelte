<script lang="ts">
  import { Flag, Pencil, PartyPopper, Plus, Target } from 'lucide-svelte';
  import { ApiError, goalApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
  import type { GoalOverview } from '$lib/types';

  /**
   * Das Familienziel: „300 Punkte zusammen, dann gibt es den Pizza-Abend".
   *
   * Die Rangliste ist ein Wettrennen, und das Jüngste liegt darin immer
   * hinten. Hier zählt jeder Punkt für alle. Die Beiträge stehen nebeneinander,
   * nach Person, nicht nach Höhe sortiert — es geht nicht darum, wer am meisten
   * beigetragen hat, sondern dass jeder etwas beigetragen hat.
   */
  let daten = $state<GoalOverview | null>(null);
  let fehler = $state('');

  const admin = $derived($session.user?.role === 'admin');
  const ziel = $derived(daten?.goal ?? null);
  const fortschritt = $derived(daten?.progress ?? 0);
  const anteil = $derived(ziel ? Math.min(100, (fortschritt / ziel.target) * 100) : 0);
  const geschafft = $derived(!!ziel?.reached_at || (ziel ? fortschritt >= ziel.target : false));
  const fehlt = $derived(ziel ? Math.max(0, ziel.target - fortschritt) : 0);

  async function laden() {
    try {
      daten = await goalApi.get();
      fehler = '';
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Konnte das Ziel nicht laden';
    }
  }

  // Nach jeder Punktebuchung irgendwo in der App lädt die Rangliste neu —
  // dann stimmt auch der Fortschritt hier nicht mehr.
  $effect(() => {
    void board.scores;
    void laden();
  });


  // ---- Setzen und ändern (Eltern) ----
  const SYMBOLE = ['🎯', '🍕', '🎬', '🏊', '🎢', '🍦', '🏕️', '🎳', '🦁', '🚲', '🎮', '🎁'];
  const ZIELE = [100, 200, 300, 500];
  let offen = $state(false);
  let neu = $state(true);
  let entwurf = $state({ title: '', emoji: '🎯', target: 300 });

  function setzen() {
    neu = true;
    entwurf = { title: '', emoji: '🍕', target: 300 };
    offen = true;
  }

  function aendern() {
    if (!ziel) return;
    neu = false;
    entwurf = { title: ziel.title, emoji: ziel.emoji, target: ziel.target };
    offen = true;
  }

  async function speichern(event: SubmitEvent) {
    event.preventDefault();
    try {
      if (neu || !ziel) await goalApi.create(entwurf);
      else await goalApi.update(ziel.id, entwurf);
      offen = false;
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht speichern', { ton: 'fehler' });
    }
  }

  async function abschliessen() {
    if (!ziel) return;
    const ok = await confirmAction({
      title: geschafft ? `${ziel.emoji} ${ziel.title} eingelöst?` : `„${ziel.title}" aufgeben?`,
      message: geschafft
        ? 'Das Ziel wandert zu den geschafften. Danach könnt ihr ein neues setzen.'
        : 'Das Ziel wird beendet, ohne geschafft zu sein. Die Punkte aller bleiben, wie sie sind.',
      confirmLabel: geschafft ? 'Eingelöst' : 'Aufgeben',
      danger: !geschafft,
    });
    if (!ok) return;
    try {
      await goalApi.close(ziel.id);
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht abschliessen', { ton: 'fehler' });
    }
  }
</script>

{#snippet zeile()}
  {#if !ziel}
    Gemeinsam auf etwas hinarbeiten
  {:else if geschafft}
    Geschafft! 🎉
  {:else}
    Noch {fehlt} Punkte
  {/if}
{/snippet}

{#snippet aktionen()}
  {#if admin}
    {#if ziel}
      <button class="btn-ghost px-3 text-muted-foreground" onclick={aendern} aria-label="Ziel ändern">
        <Pencil class="h-4 w-4" />
      </button>
    {:else}
      <button class="btn-primary px-3" onclick={setzen} aria-label="Familienziel setzen">
        <Plus class="h-5 w-5" />
      </button>
    {/if}
  {/if}
{/snippet}

<Kachel ton="var(--ton-ziel)" titel="Familienziel" icon={Target} {zeile} {aktionen} {fehler}>
  {#if !ziel}
    <KachelLeer
      icon={Flag}
      titel="Noch kein gemeinsames Ziel"
      hinweis={admin
        ? 'Zum Beispiel: 300 Punkte zusammen, dann gibt es den Pizza-Abend. Jeder Punkt zählt für alle.'
        : 'Ein Elternteil setzt ein Ziel, auf das alle zusammen hinarbeiten.'}
    >
      {#snippet aktion()}
        {#if admin}
          <button class="btn-primary text-sm" onclick={setzen}>Ziel setzen</button>
        {/if}
      {/snippet}
    </KachelLeer>
  {:else}
    <div class="flex flex-1 flex-col gap-4">
      <div class="flex items-center gap-4">
        <span
          class="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl bg-muted text-4xl
            {geschafft ? 'animate-pop' : ''}"
        >
          {ziel.emoji}
        </span>
        <div class="min-w-0">
          <p class="truncate text-lg font-semibold leading-tight">{ziel.title}</p>
          <p class="font-display text-3xl font-light leading-none tabular-nums">
            {fortschritt}<span class="text-base text-muted-foreground"> / {ziel.target}</span>
          </p>
        </div>
      </div>

      <!--
        Der Balken besteht aus den Beiträgen, jeder in seiner Farbe. So sieht
        man auf einen Blick, dass alle daran gebaut haben — ohne Rangfolge.
      -->
      <div
        class="flex h-4 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-valuenow={Math.round(anteil)}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Fortschritt zum Familienziel"
      >
        {#each daten?.contributions ?? [] as b (b.user_id)}
          <div
            class="h-full transition-[width] duration-700 first:rounded-l-full"
            style="width: {ziel.target > 0 ? Math.min(100, (b.points / ziel.target) * 100) : 0}%; background-color: {b.color}"
            title="{b.name}: {b.points}"
          ></div>
        {/each}
      </div>

      {#if (daten?.contributions ?? []).length > 0}
        <ul class="flex flex-wrap gap-1.5">
          {#each daten?.contributions ?? [] as b (b.user_id)}
            <li class="chip !min-h-[30px] !gap-1 !px-2.5 text-xs">
              <span class="h-2 w-2 rounded-full" style="background-color: {b.color}"></span>
              {b.avatar_emoji} {b.points}
            </li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-muted-foreground">Der erste Punkt fehlt noch — wer fängt an?</p>
      {/if}

      {#if geschafft}
        <div class="mt-auto flex items-center gap-3 rounded-xl bg-success/10 p-3 text-sm text-success">
          <PartyPopper class="h-5 w-5 shrink-0" />
          <span class="min-w-0 flex-1">Geschafft — zusammen! Jetzt ist {ziel.title} dran.</span>
          {#if admin}
            <button class="btn-primary shrink-0 px-3 text-sm" onclick={abschliessen}>Eingelöst</button>
          {/if}
        </div>
      {/if}

      {#if (daten?.reached ?? []).length > 0}
        <p class="mt-auto text-xs text-muted-foreground">
          Schon geschafft:
          {#each daten?.reached ?? [] as g, i (g.id)}{i > 0 ? ' · ' : ' '}{g.emoji} {g.title}{/each}
        </p>
      {/if}
    </div>
  {/if}
</Kachel>

<Modal bind:open={offen} title={neu ? 'Familienziel setzen' : 'Familienziel ändern'}>
  <form class="space-y-4" onsubmit={speichern}>
    <input class="input" placeholder="Worauf spart ihr? z. B. Pizza-Abend" bind:value={entwurf.title} maxlength="80" />
    <div class="grid grid-cols-6 gap-1">
      {#each SYMBOLE as s (s)}
        <button
          type="button"
          class="flex h-11 items-center justify-center rounded-lg text-2xl transition-colors
            {entwurf.emoji === s ? 'bg-primary/15 ring-2 ring-primary' : 'hover:bg-accent'}"
          onclick={() => (entwurf.emoji = s)}
          aria-label="Symbol {s}"
          aria-pressed={entwurf.emoji === s}
        >
          {s}
        </button>
      {/each}
    </div>
    <div>
      <p class="mb-1.5 text-xs text-muted-foreground">Wie viele Punkte zusammen?</p>
      <div class="flex flex-wrap items-center gap-1.5">
        {#each ZIELE as z (z)}
          <button type="button" class="chip font-semibold tabular-nums" aria-pressed={entwurf.target === z} onclick={() => (entwurf.target = z)}>
            {z}
          </button>
        {/each}
        <input class="input !w-28 !py-2 text-center" type="number" min="1" max="100000" bind:value={entwurf.target} aria-label="Andere Zahl" />
      </div>
      <p class="mt-1.5 text-xs text-muted-foreground">
        Zum Vergleich: Eine Woche Aufgaben bringt einer Familie oft 150 bis 300 Punkte.
        Abzüge zählen nicht dagegen.
      </p>
    </div>
    <div class="flex gap-2 pt-1">
      {#if !neu}
        <button type="button" class="btn-outline px-3 text-sm" onclick={() => { offen = false; void abschliessen(); }}>
          {geschafft ? 'Eingelöst' : 'Aufgeben'}
        </button>
      {/if}
      <button class="btn-primary flex-1" disabled={!entwurf.title.trim() || entwurf.target < 1}>
        {neu ? 'Ziel setzen' : 'Speichern'}
      </button>
    </div>
    {#if neu && ziel}
      <p class="text-xs text-muted-foreground">Das laufende Ziel „{ziel.title}" wird dabei beendet.</p>
    {/if}
  </form>
</Modal>
