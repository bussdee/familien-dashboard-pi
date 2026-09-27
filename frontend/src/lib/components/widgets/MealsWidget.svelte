<script lang="ts">
  import { onMount } from 'svelte';
  import { ArrowRight, UtensilsCrossed } from 'lucide-svelte';
  import { addDays, format, isToday, isTomorrow } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { mealsApi } from '$lib/api';
  import type { Meal } from '$lib/types';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';

  /**
   * Heute und die nächsten Tage aus dem Essensplan. Bearbeitet wird auf der
   * eigenen Seite — in einer Kachel wäre die Woche zu eng.
   */
  let { meals = $bindable([]) }: { meals?: Meal[] } = $props();

  let fehler = $state('');

  const tage = $derived(
    Array.from({ length: 4 }, (_, i) => {
      const d = addDays(new Date(), i);
      const key = format(d, 'yyyy-MM-dd');
      return { key, d, meal: meals.find((m) => m.day === key) ?? null };
    }),
  );
  const heute = $derived(tage[0]?.meal ?? null);

  function tagName(d: Date) {
    if (isToday(d)) return 'Heute';
    if (isTomorrow(d)) return 'Morgen';
    return format(d, 'EEEE', { locale: de });
  }

  onMount(() => {
    if (meals.length > 0) return;
    mealsApi
      .list(format(new Date(), 'yyyy-MM-dd'), 4)
      .then((r) => (meals = r.meals))
      .catch(() => (fehler = 'Konnte den Essensplan nicht laden'));
  });
</script>

{#snippet zeile()}
  {#if heute}Heute: {heute.title}{:else}Heute noch nichts geplant{/if}
{/snippet}

{#snippet aktionen()}
  <a href="/essen" class="btn-ghost px-3 text-sm text-muted-foreground" aria-label="Zum Essensplan">
    Woche <ArrowRight class="h-4 w-4" />
  </a>
{/snippet}

<Kachel ton="var(--ton-essen)" titel="Essensplan" icon={UtensilsCrossed} {zeile} {aktionen} {fehler}>
  {#if tage.every((t) => !t.meal)}
    <KachelLeer
      icon={UtensilsCrossed}
      titel="Was gibt's diese Woche?"
      hinweis="Gerichte eintragen, die Zutaten wandern mit einem Tipp auf die Einkaufsliste."
    >
      {#snippet aktion()}
        <a href="/essen?heute=1" class="btn-primary text-sm">Heute planen</a>
      {/snippet}
    </KachelLeer>
  {:else}
    <ul class="space-y-1">
      {#each tage as t (t.key)}
        <li>
          <a
            href="/essen"
            class="flex items-center gap-3 rounded-xl px-2 py-2 transition-colors hover:bg-muted/30
              {isToday(t.d) ? 'bg-muted/40' : ''}"
          >
            <span class="w-16 shrink-0 text-xs font-medium uppercase tracking-wider {isToday(t.d) ? 'text-primary' : 'text-muted-foreground'}">
              {tagName(t.d)}
            </span>
            {#if t.meal}
              <span class="min-w-0 flex-1 truncate {isToday(t.d) ? 'text-base font-semibold' : 'text-sm'}">
                {t.meal.title}
              </span>
            {:else}
              <span class="min-w-0 flex-1 truncate text-sm text-muted-foreground/70">—</span>
            {/if}
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</Kachel>

