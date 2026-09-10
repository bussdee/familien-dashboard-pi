<script lang="ts">
  import { onMount } from 'svelte';
  import { Clock, House, Pencil } from 'lucide-svelte';
  import { timesApi } from '$lib/api';
  import { session } from '$lib/stores';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
  import type { TimeBlock, TimeDay, TimeOverview } from '$lib/types';

  let daten = $state<TimeOverview | null>(null);
  let loading = $state(true);

  /** Wie viele Tage die Kachel zeigt. Heute plus die nächsten zwei. */
  const TAGE = 3;

  const heute = $derived(daten?.days?.[0] ?? null);
  const weitere = $derived((daten?.days ?? []).slice(1));

  // Nur wer wirklich unterwegs ist, steht in der Kachel. „Frei" und „Urlaub"
  // sind Nicht-Termine — sie heben das Wochenmuster auf, aber niemand muss
  // dafür auf die Uhr sehen.
  const unterwegs = (tag: TimeDay | null) =>
    (tag?.blocks ?? []).filter((b) => ['arbeit', 'schule', 'sonstiges'].includes(b.kind));

  const kurz = (iso: string) =>
    new Date(iso + 'T12:00:00').toLocaleDateString('de-DE', { weekday: 'short' });

  /**
   * Wie eine Zeit dasteht. Bei einer Nachtschicht ist die nackte Angabe
   * „20:00–07:00" irreführend — sie liest sich wie eine Zeitspanne am selben
   * Tag. Deshalb steht dazu, wohin sie reicht.
   */
  const spanne = (b: TimeBlock) => {
    if (!b.start_time || !b.end_time) return 'den ganzen Tag';
    if (b.continues_tomorrow) return `ab ${b.start_time}, bis morgen ${b.end_time}`;
    if (b.from_yesterday) return `seit gestern, bis ${b.end_time}`;
    return `${b.start_time}–${b.end_time}`;
  };

  async function laden() {
    try {
      daten = await timesApi.overview(undefined, TAGE);
    } catch {
      daten = null;
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void laden();
    // Um Mitternacht ist „heute" ein anderer Tag. Ein Wandtablet läuft
    // wochenlang durch, ohne dass jemand neu lädt.
    const timer = setInterval(() => void laden(), 30 * 60_000);
    return () => clearInterval(timer);
  });
</script>

{#snippet zeile()}
  {#if loading}
    Lade…
  {:else if unterwegs(heute).length === 0}
    Heute sind alle da
  {:else if heute?.all_home_from}
    Ab {heute.all_home_from} sind alle da
  {:else}
    {unterwegs(heute).length}
    {unterwegs(heute).length === 1 ? 'Person' : 'Personen'} unterwegs
  {/if}
{/snippet}

{#snippet aktionen()}
  <!-- Eintragen gehört einer Person. Am Wandgerät führt der Weg nur in eine
       Sperre, deshalb dort kein Knopf. -->
  {#if !$session.device}
    <a class="btn-ghost px-2 text-muted-foreground" href="/zeiten" aria-label="Zeiten eintragen">
      <Pencil class="h-4 w-4" />
    </a>
  {/if}
{/snippet}

<Kachel titel="Arbeit & Schule" icon={Clock} {zeile} {aktionen}>
  {#if loading}
    <div class="space-y-2">
      {#each Array(3) as _, i (i)}
        <div class="h-10 animate-pulse rounded-lg bg-muted/30"></div>
      {/each}
    </div>
  {:else if (daten?.days ?? []).every((t) => unterwegs(t).length === 0)}
    <KachelLeer
      icon={House}
      titel="Keine Zeiten eingetragen"
      hinweis={$session.device
        ? 'Wer seinen Stundenplan oder seine Schichten einträgt, sieht hier, wann alle da sind.'
        : 'Trag deinen Stundenplan oder deine Schichten ein — dann steht hier, ab wann alle da sind.'}
    >
      {#snippet aktion()}
        {#if !$session.device}
          <a href="/zeiten" class="btn-outline text-sm">Zeiten eintragen</a>
        {/if}
      {/snippet}
    </KachelLeer>
  {:else}
    <!-- Heute ausführlich, die nächsten Tage nur als Zeile. -->
    {#if unterwegs(heute).length > 0}
      <ul class="space-y-1.5">
        {#each unterwegs(heute) as b (b.user_id + b.start_time + b.kind)}
          <li class="flex items-center gap-3 rounded-lg bg-muted/30 px-2.5 py-2">
            <span
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-lg"
              style="background-color: {b.user_color}22"
            >
              {b.user_emoji}
            </span>
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm font-medium">{b.user_name}</span>
              <span class="block truncate text-xs text-muted-foreground">
                {b.kind === 'schule' ? 'Schule' : b.kind === 'arbeit' ? 'Arbeit' : 'Unterwegs'}
                {#if b.continues_tomorrow}· Nachtschicht{/if}
                {#if b.note}· {b.note}{/if}
              </span>
            </span>
            <span class="shrink-0 text-sm tabular-nums">{spanne(b)}</span>
          </li>
        {/each}
      </ul>
    {:else}
      <p class="rounded-lg bg-success/10 px-3 py-2.5 text-sm text-success">
        Heute sind alle da.
      </p>
    {/if}

    {#if heute?.all_home_from}
      <p class="mt-3 flex items-center gap-2 rounded-lg bg-primary/10 px-3 py-2.5 text-sm text-primary">
        <House class="h-4 w-4 shrink-0" />
        Ab <strong>{heute.all_home_from}</strong> sind heute alle zu Hause.
      </p>
    {:else if unterwegs(heute).some((b) => b.continues_tomorrow)}
      <!--
        Bei einer Nachtschicht gibt es keine Rückkehrzeit an diesem Tag. Das
        zu sagen ist ehrlicher, als die Zeile wegzulassen — sonst sucht man
        nach ihr.
      -->
      <p class="mt-3 flex items-center gap-2 rounded-lg bg-muted/40 px-3 py-2.5 text-sm text-muted-foreground">
        <House class="h-4 w-4 shrink-0" />
        Heute kommt nicht mehr jeder zurück — Nachtschicht.
      </p>
    {/if}

    {#if weitere.length > 0}
      <ul class="mt-3 space-y-1 border-t border-[color:var(--haarlinie)] pt-3">
        {#each weitere as tag (tag.date)}
          <li class="flex items-baseline gap-2 text-xs">
            <span class="w-8 shrink-0 font-medium text-muted-foreground">{kurz(tag.date)}</span>
            <span class="min-w-0 flex-1 truncate text-muted-foreground">
              {#if unterwegs(tag).length === 0}
                alle da
              {:else}
                {unterwegs(tag)
                  .map((b) =>
                    b.continues_tomorrow
                      ? `${b.user_emoji} ab ${b.start_time}`
                      : b.from_yesterday
                        ? `${b.user_emoji} bis ${b.end_time}`
                        : `${b.user_emoji} ${b.start_time || ''}–${b.end_time || ''}`,
                  )
                  .join(' · ')}
              {/if}
            </span>
            {#if tag.all_home_from}
              <span class="shrink-0 tabular-nums text-muted-foreground">ab {tag.all_home_from}</span>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</Kachel>
