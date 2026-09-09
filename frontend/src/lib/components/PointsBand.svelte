<script lang="ts">
  import { Flame, Trophy } from 'lucide-svelte';
  import type { Score } from '$lib/types';

  let { me, total = 0 }: { me: Score | null; total?: number } = $props();

  const platz = $derived(
    me && total > 0 ? `Platz ${me.rank} von ${total}` : null,
  );
</script>

{#if me}
  <!--
    Der Punktestand als durchgehendes Band quer über die Seite. Vorher steckte
    er klein in der Aufgaben-Kachel; aus dem Flur war er nicht zu sehen, und
    genau das soll er ja: anspornen.
  -->
  <a
    href="/rangliste"
    class="punkte-band flex items-center gap-4 border-y border-[color:var(--haarlinie)] py-4 transition-colors hover:bg-muted/20 sm:gap-6"
  >
    <span class="font-display text-4xl font-medium leading-none tracking-tight sm:text-5xl">
      {me.total_points}
    </span>

    <span class="min-w-0 flex-1">
      <span class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <span class="text-sm">
          Level {me.level} · {me.level_name}
          {#if platz}<span class="font-light text-muted-foreground">· {platz}</span>{/if}
        </span>
        <span class="text-xs font-light text-muted-foreground">
          noch {me.points_to_next} bis Level {me.level + 1}
        </span>
      </span>
      <span class="mt-2 block h-[5px] overflow-hidden rounded-full bg-muted">
        <span
          class="block h-full rounded-full bg-gradient-to-r from-primary to-amber-400 transition-[width] duration-500"
          style="width: {Math.min(100, Math.max(0, me.level_progress))}%"
        ></span>
      </span>
    </span>

    {#if me.streak_days >= 2}
      <span class="flex shrink-0 items-center gap-1.5 text-xs text-amber-500 dark:text-amber-400">
        <Flame class="h-3.5 w-3.5" />
        <span class="hidden sm:inline">{me.streak_days} Tage in Folge</span>
        <span class="sm:hidden">{me.streak_days}</span>
      </span>
    {/if}

    {#if me.badges.length > 0}
      <span class="hidden shrink-0 items-center gap-1 text-sm md:flex" title="Abzeichen">
        {#each me.badges.slice(0, 3) as badge (badge.id)}
          <span>{badge.emoji}</span>
        {/each}
      </span>
    {:else if me.rank === 1 && me.total_points > 0}
      <Trophy class="hidden h-4 w-4 shrink-0 text-amber-500 md:block dark:text-amber-400" />
    {/if}
  </a>
{/if}
