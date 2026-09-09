<script lang="ts">
  import { Cake, CalendarHeart, PartyPopper, Plane, Timer } from 'lucide-svelte';
  import { differenceInCalendarDays, format, parseISO, startOfDay } from 'date-fns';
  import { de } from 'date-fns/locale';
  import type { CalendarEvent } from '$lib/types';

  let { events = [] }: { events?: CalendarEvent[] } = $props();

  // Keywords that turn an ordinary calendar entry into a countdown.
  const kinds = [
    { match: ['geburtstag', 'birthday', 'geburi'], icon: Cake, color: '#ec4899' },
    { match: ['ferien', 'urlaub', 'reise', 'flug'], icon: Plane, color: '#f59e0b' },
    { match: ['jubiläum', 'hochzeit', 'jahrestag'], icon: CalendarHeart, color: '#ef4444' },
    { match: ['feier', 'party', 'fest', 'weihnachten', 'ostern', 'silvester'], icon: PartyPopper, color: '#8b5cf6' },
  ];

  const countdowns = $derived.by(() => {
    const today = startOfDay(new Date());
    return events
      .map((event) => {
        const title = event.title.toLowerCase();
        const kind = kinds.find((k) => k.match.some((m) => title.includes(m)));
        if (!kind) return null;
        const days = differenceInCalendarDays(parseISO(event.start), today);
        if (days < 0) return null;
        return { event, days, icon: kind.icon, color: kind.color };
      })
      .filter((x): x is NonNullable<typeof x> => x !== null)
      .sort((a, b) => a.days - b.days)
      .slice(0, 5);
  });

  const label = (days: number) =>
    days === 0 ? 'Heute!' : days === 1 ? 'Morgen' : `in ${days} Tagen`;
</script>

<section class="flaeche">
  <header class="mb-4 flex items-center justify-between">
    <div>
      <h2 class="text-lg font-semibold">Countdowns</h2>
      <p class="text-sm text-muted-foreground">Worauf wir uns freuen</p>
    </div>
    <Timer class="h-5 w-5 text-muted-foreground" />
  </header>

  {#if countdowns.length === 0}
    <div class="py-8 text-center text-muted-foreground">
      <PartyPopper class="mx-auto mb-2 h-10 w-10 opacity-40" />
      <p class="text-sm">Keine Countdowns</p>
      <p class="mt-1 text-xs">
        Termine mit „Geburtstag“, „Ferien“ oder „Feier“ erscheinen hier automatisch
      </p>
    </div>
  {:else}
    <div class="space-y-2">
      {#each countdowns as item (item.event.id)}
        {@const Icon = item.icon}
        <div class="flex items-center gap-3 rounded-lg px-1 py-2.5">
          <div
            class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl"
            style="background-color: {item.color}1a"
          >
            <Icon class="h-5 w-5" style="color: {item.color}" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{item.event.title}</p>
            <p class="text-xs text-muted-foreground">
              {format(parseISO(item.event.start), 'EEEE, d. MMMM', { locale: de })}
            </p>
          </div>
          <div class="shrink-0 text-right">
            <p class="text-xl font-bold tabular-nums" style="color: {item.color}">
              {item.days}
            </p>
            <p class="text-[11px] text-muted-foreground">{label(item.days)}</p>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
