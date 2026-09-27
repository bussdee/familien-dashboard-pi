<script lang="ts">
  import { onMount } from 'svelte';
  import { formatDistanceToNow, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { CalendarRange, Flame, ShoppingCart, Sparkles, Star, Trophy } from 'lucide-svelte';
  import { board, sourceLabels } from '$lib/stores/scores.svelte';
  import { scoreApi } from '$lib/api';
  import { session } from '$lib/stores';
  import LevelBar from '$lib/components/LevelBar.svelte';
  import type { MonthBoard } from '$lib/types';

  let loading = $state(!board.loaded);

  /**
   * Die Monatsranglisten. Die Gesamtwertung oben wächst immer weiter — wer
   * im März angefangen hat, holt den Vorsprung nie mehr auf. Ein Monat
   * dagegen fängt für alle bei null an, und sein Ergebnis bleibt stehen,
   * auch wenn die einmaligen Aufgaben von damals gelöscht wurden.
   */
  let monate = $state<MonthBoard[]>([]);
  /** Welcher Monat aufgeklappt ist. Der erste ist es von Anfang an. */
  let offen = $state<string | null>(null);

  const medaille = (platz: number) => (platz <= 3 ? ['🥇', '🥈', '🥉'][platz - 1] : '');

  /**
   * Die Zeile unter dem Monatsnamen. Bei Gleichstand stehen beide da —
   * einen von zweien zum Sieger zu erklären wäre der sicherste Weg zu
   * Streit am Frühstückstisch.
   */
  function siegerzeile(monat: MonthBoard): string {
    const vorn = monat.ranks.filter((r) => r.rank === 1 && r.points > 0);
    if (vorn.length === 0) return 'Noch keine Punkte';
    const namen = vorn.map((r) => `${r.avatar_emoji} ${r.name}`).join(' und ');
    const wort = monat.running ? 'Vorn' : vorn.length > 1 ? 'Geteilt gewonnen' : 'Gewonnen';
    return `${wort}: ${namen} · ${vorn[0].points} Punkte`;
  }

  const podium = $derived(board.podium);
  const scoring = $derived(podium.filter((s) => s.total_points > 0));
  // A podium only means something once at least two people are on the board.
  const showPodium = $derived(scoring.length >= 2);
  const top3 = $derived(scoring.slice(0, 3));
  const stage = $derived([top3[1], top3[0], top3[2]].filter(Boolean));
  const rest = $derived(podium.slice(3));
  const me = $derived(board.for($session.user?.id));

  const medals = ['🥇', '🥈', '🥉'];
  const heights = { 1: 'h-24', 2: 'h-16', 3: 'h-12' } as const;

  const relative = (iso: string) =>
    formatDistanceToNow(parseISO(iso), { addSuffix: true, locale: de });

  onMount(async () => {
    try {
      await board.refresh();
    } finally {
      loading = false;
    }
    try {
      monate = await scoreApi.months();
      offen = monate[0]?.month ?? null;
    } catch {
      // Ohne Monatsranglisten bleibt der Rest der Seite brauchbar.
    }
  });
</script>

<svelte:head><title>Rangliste · Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-3xl px-4 py-5">
  <header class="mb-6 flex flex-wrap items-end justify-between gap-3">
    <div>
      <h1 class="seiten-titel flex items-center gap-2">
        <Trophy class="h-7 w-7 text-amber-500" /> Rangliste
      </h1>
      <p class="mt-1 text-sm text-muted-foreground">
        Punkte gibt es für erledigte Aufgaben und fürs Einkaufen.
      </p>
    </div>
    <a href="/belohnungen" class="chip">🎁 Punkte eintauschen →</a>
  </header>

  {#if loading}
    <div class="card h-64 animate-pulse bg-muted/40"></div>
  {:else if podium.every((s) => s.total_points === 0)}
    <section class="card p-8 text-center">
      <Sparkles class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-40" />
      <p class="font-medium">Noch keine Punkte</p>
      <p class="mt-1 text-sm text-muted-foreground">
        Hake eine Aufgabe ab oder erledige den Einkauf – dann geht es los.
      </p>
      <a href="/" class="btn-primary mt-4 inline-flex">Zur Übersicht</a>
    </section>
  {:else}
    <!-- Podium -->
    {#if showPodium}
      <section class="card mb-4 overflow-hidden p-5">
        <div class="flex items-end justify-center gap-3 sm:gap-6">
          {#each stage as score (score.id)}
          <div class="flex w-24 flex-col items-center sm:w-28">
            <span class="mb-1 text-3xl">{score.avatar_emoji}</span>
            <p class="max-w-full truncate text-sm font-semibold">{score.name}</p>
            <p class="text-xs tabular-nums text-muted-foreground">{score.total_points} P</p>
            <div
              class="mt-2 flex w-full items-start justify-center rounded-t-lg pt-2 text-2xl
                {heights[score.rank as 1 | 2 | 3] ?? 'h-12'}
                {score.rank === 1
                ? 'bg-amber-400/25'
                : score.rank === 2
                  ? 'bg-slate-400/25'
                  : 'bg-orange-500/20'}"
            >
              {medals[score.rank - 1] ?? score.rank}
            </div>
          </div>
        {/each}
        </div>
      </section>
    {:else}
      <section class="card mb-4 flex items-center gap-3 p-5">
        <span class="text-3xl">{podium[0]?.avatar_emoji ?? '🏆'}</span>
        <div>
          <p class="font-semibold">{podium[0]?.name} führt mit {podium[0]?.total_points} Punkten</p>
          <p class="text-sm text-muted-foreground">
            Sobald jemand mitzieht, gibt es hier ein Siegertreppchen.
          </p>
        </div>
      </section>
    {/if}

    <!-- Full table -->
    <section class="card mb-4 divide-y divide-border">
      {#each podium as score (score.id)}
        <div
          class="flex items-center gap-3 p-4 {score.id === $session.user?.id
            ? 'bg-primary/5'
            : ''}"
        >
          <span class="w-7 shrink-0 text-center text-sm font-semibold tabular-nums text-muted-foreground">
            {score.total_points > 0 ? (medals[score.rank - 1] ?? score.rank) : '–'}
          </span>
          <span class="text-2xl">{score.avatar_emoji}</span>

          <div class="min-w-0 flex-1">
            <p class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium">
              {score.name}
              {#if score.id === $session.user?.id}
                <span class="rounded-full bg-primary/15 px-2 py-0.5 text-[10px] text-primary">du</span>
              {/if}
              {#if score.streak_days >= 3}
                <span class="flex items-center gap-0.5 text-[11px] text-orange-500">
                  <Flame class="h-3 w-3" />{score.streak_days} Tage
                </span>
              {/if}
            </p>
            <div class="mt-1.5 max-w-[220px]">
              <LevelBar {score} showLabel={false} size="sm" />
            </div>
            <p class="mt-1 text-[11px] text-muted-foreground">
              Level {score.level} · {score.level_name} · diese Woche {score.this_week} P
            </p>
          </div>

          <div class="shrink-0 text-right">
            <p class="text-lg font-bold tabular-nums">{score.total_points}</p>
            <p class="text-[11px] text-muted-foreground">Punkte</p>
          </div>
        </div>
      {/each}
    </section>

    <!-- Badges -->
    {#if me && me.badges.length > 0}
      <section class="card mb-4 p-5">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Deine Abzeichen
        </h2>
        <div class="flex flex-wrap gap-2">
          {#each me.badges as badge (badge.id)}
            <span
              class="flex items-center gap-2 rounded-full bg-muted px-3 py-1.5 text-xs"
              title={badge.description}
            >
              <span class="text-base leading-none">{badge.emoji}</span>
              {badge.label}
            </span>
          {/each}
        </div>
      </section>
    {/if}

    <!-- Everyone's badges, so there is something to aim for -->
    {#if rest.length > 0 || podium.some((s) => s.badges.length > 0)}
      <section class="card mb-4 p-5">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Abzeichen der Familie
        </h2>
        <div class="space-y-2">
          {#each podium.filter((s) => s.badges.length > 0) as score (score.id)}
            <div class="flex flex-wrap items-center gap-2">
              <span class="w-24 shrink-0 truncate text-sm">
                {score.avatar_emoji}
                {score.name}
              </span>
              {#each score.badges as badge (badge.id)}
                <span class="text-lg" title="{badge.label} – {badge.description}">
                  {badge.emoji}
                </span>
              {/each}
            </div>
          {/each}
        </div>
      </section>
    {/if}

    <!--
      Die Monate. Der laufende steht oben und ist als Zwischenstand
      gekennzeichnet; darunter die abgeschlossenen, die sich nicht mehr
      ändern. Aufgeklappt ist immer nur einer — sonst wird die Seite auf dem
      Handy eine Tapete.
    -->
    {#if monate.length > 0}
      <section class="card mb-4">
        <div class="p-5 pb-3">
          <h2 class="flex items-center gap-2 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
            <CalendarRange class="h-4 w-4" /> Monatswertung
          </h2>
          <p class="mt-1 text-xs text-muted-foreground">
            Jeder Monat fängt bei null an. Abgeschlossene Monate bleiben stehen.
          </p>
        </div>

        <div class="divide-y divide-border border-t border-border">
          {#each monate as monat (monat.month)}
            {@const auf = offen === monat.month}

            <div>
              <button
                class="flex w-full items-center gap-3 p-4 text-left transition-colors hover:bg-muted/25"
                onclick={() => (offen = auf ? null : monat.month)}
                aria-expanded={auf}
              >
                <span class="min-w-0 flex-1">
                  <span class="block text-sm font-medium">
                    {monat.label}
                    {#if monat.running}
                      <span class="ml-1 text-xs font-normal text-muted-foreground">läuft noch</span>
                    {/if}
                  </span>
                  <span class="mt-0.5 block truncate text-xs text-muted-foreground">
                    {siegerzeile(monat)}
                  </span>
                </span>
                <span class="shrink-0 text-xs text-muted-foreground">
                  {auf ? 'Zuklappen' : 'Ansehen'}
                </span>
              </button>

              {#if auf}
                <ul class="space-y-1 px-4 pb-4">
                  {#each monat.ranks as rang (rang.user_id)}
                    <li class="flex items-center gap-3 rounded-lg bg-muted/30 px-3 py-2">
                      <span class="w-7 shrink-0 text-center text-sm tabular-nums">
                        {medaille(rang.rank) || rang.rank + '.'}
                      </span>
                      <span class="text-lg">{rang.avatar_emoji}</span>
                      <span class="min-w-0 flex-1 truncate text-sm">{rang.name}</span>
                      <span class="shrink-0 text-[11px] text-muted-foreground">
                        {rang.activities}×
                      </span>
                      <span class="shrink-0 text-sm font-semibold tabular-nums text-primary">
                        {rang.points}
                      </span>
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          {/each}
        </div>
      </section>
    {/if}

    <!-- Activity feed -->
    {#if board.history.length > 0}
      <section class="card p-5">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Zuletzt passiert
        </h2>
        <ul class="space-y-2">
          {#each board.history as item (item.id)}
            <li class="flex items-center gap-3 rounded-lg bg-muted/30 p-2.5">
              <span class="text-lg">{item.user_emoji}</span>
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm">
                  <span class="font-medium">{item.user_name}</span>
                  {#if item.source === 'shopping'}
                    <ShoppingCart class="mx-1 inline h-3 w-3 text-muted-foreground" />
                  {:else}
                    <Star class="mx-1 inline h-3 w-3 text-muted-foreground" />
                  {/if}
                  {item.note || sourceLabels[item.source]?.label || item.source}
                </p>
                <p class="text-[11px] text-muted-foreground">{relative(item.created_at)}</p>
              </div>
              <span class="shrink-0 text-sm font-semibold tabular-nums text-primary">
                +{item.points}
              </span>
            </li>
          {/each}
        </ul>
      </section>
    {/if}
  {/if}
</div>
