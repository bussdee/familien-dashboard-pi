<script lang="ts">
  import { format, isSameDay, parseISO } from 'date-fns';
  import { CalendarDays, House, ListChecks, ShoppingCart, UtensilsCrossed } from 'lucide-svelte';
  import type { CalendarEvent, Chore, Meal, ShoppingItem, TimeDay } from '$lib/types';

  /**
   * Der Tag auf einen Blick: fünf Antworten, die sonst fünf Fragen wären.
   * Was steht an? Was gibt's zu essen? Was ist noch zu tun? Was fehlt? Ab
   * wann sind alle da?
   *
   * Bis 1.7 stand hier eine Textzeile mit zwei Zahlen. Die Leiste sagt
   * dasselbe und mehr, und jede Karte führt dorthin, wo man etwas daran
   * ändern kann.
   */
  let {
    events = [],
    chores = [],
    shopping = [],
    meals = [],
    zeiten = null,
    meineId = null,
  }: {
    events?: CalendarEvent[];
    chores?: Chore[];
    shopping?: ShoppingItem[];
    meals?: Meal[];
    zeiten?: TimeDay | null;
    meineId?: number | null;
  } = $props();

  // „Jetzt" wird bei jeder Aktualisierung neu bestimmt, nicht einmal beim
  // Laden: Ein Wandtablet läuft über Nacht, und die Übersicht lädt ihre
  // Daten alle fünf Minuten neu — damit rechnen auch diese Zeilen neu.
  const termin = $derived.by(() => {
    const jetzt = new Date();
    const kommende = events
      .filter((e) => {
        const ende = parseISO(e.end);
        return ende >= jetzt && isSameDay(parseISO(e.start), jetzt);
      })
      .sort((a, b) => a.start.localeCompare(b.start));
    return { erster: kommende[0] ?? null, anzahl: kommende.length };
  });

  const essen = $derived.by(() => {
    const heuteKey = format(new Date(), 'yyyy-MM-dd');
    return meals.find((m) => m.day === heuteKey) ?? null;
  });
  const faellig = $derived(chores.filter((c) => c.is_due));
  const fuerMich = $derived(faellig.filter((c) => meineId !== null && c.assignee_id === meineId).length);
  const offenEinkauf = $derived(shopping.filter((i) => !i.checked).length);

  const terminText = $derived.by(() => {
    const e = termin.erster;
    if (!e) return 'Keine Termine mehr';
    const zeit = e.all_day ? 'Ganztags' : format(parseISO(e.start), 'HH:mm');
    return `${zeit} ${e.title}`;
  });

  const zuhause = $derived.by(() => {
    if (!zeiten) return null;
    if (zeiten.all_home_from) return `Alle da ab ${zeiten.all_home_from}`;
    if (zeiten.blocks.length === 0) return 'Alle zu Hause';
    return null;
  });
</script>

<!--
  Auf dem Handy seitwärts wischbar, auf dem grossen Bildschirm in einer
  Reihe. snap-x, damit eine Karte nicht halb abgeschnitten stehen bleibt.

  Ohne sichtbare Scrollleiste: Sie hing als Balken unter den Karten und sah
  nach Fehler aus. Dass die Reihe weitergeht, sagt die angeschnittene Karte
  am rechten Rand — der Balken sagte es doppelt.
-->
<nav
  class="heute-reihe -mx-4 mb-6 flex snap-x gap-2.5 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0 lg:grid lg:grid-cols-5 lg:overflow-visible"
  aria-label="Heute auf einen Blick"
>
  <a href="#calendar" class="heute-karte" style="--ton: var(--ton-kalender)">
    <span class="kachel-symbol"><CalendarDays class="h-[18px] w-[18px]" /></span>
    <span class="min-w-0">
      <span class="heute-label">Termine{termin.anzahl > 1 ? ` · ${termin.anzahl}` : ''}</span>
      <span class="heute-wert">{terminText}</span>
    </span>
  </a>

  <a href="/essen" class="heute-karte" style="--ton: var(--ton-essen)">
    <span class="kachel-symbol"><UtensilsCrossed class="h-[18px] w-[18px]" /></span>
    <span class="min-w-0">
      <span class="heute-label">Essen</span>
      <span class="heute-wert {essen ? '' : 'text-muted-foreground'}">{essen?.title ?? 'Noch offen'}</span>
    </span>
  </a>

  <a href="#chores" class="heute-karte" style="--ton: var(--ton-aufgaben)">
    <span class="kachel-symbol"><ListChecks class="h-[18px] w-[18px]" /></span>
    <span class="min-w-0">
      <span class="heute-label">Aufgaben</span>
      <span class="heute-wert">
        {#if faellig.length === 0}
          Alles erledigt ✨
        {:else}
          {faellig.length} offen{fuerMich > 0 ? ` · ${fuerMich} für dich` : ''}
        {/if}
      </span>
    </span>
  </a>

  <a href="/einkaufen" class="heute-karte" style="--ton: var(--ton-einkaufen)">
    <span class="kachel-symbol"><ShoppingCart class="h-[18px] w-[18px]" /></span>
    <span class="min-w-0">
      <span class="heute-label">Einkauf</span>
      <span class="heute-wert">
        {offenEinkauf === 0 ? 'Liste ist leer' : `${offenEinkauf} ${offenEinkauf === 1 ? 'Artikel' : 'Artikel'}`}
      </span>
    </span>
  </a>

  <a href="/zeiten" class="heute-karte" style="--ton: var(--ton-zeiten)">
    <span class="kachel-symbol"><House class="h-[18px] w-[18px]" /></span>
    <span class="min-w-0">
      <span class="heute-label">Zuhause</span>
      <span class="heute-wert {zuhause ? '' : 'text-muted-foreground'}">{zuhause ?? 'Siehe Zeiten'}</span>
    </span>
  </a>
</nav>

<style>
  .heute-reihe {
    scrollbar-width: none;
  }
  .heute-reihe::-webkit-scrollbar {
    display: none;
  }

  .heute-karte {
    display: flex;
    min-width: 12.5rem;
    scroll-snap-align: start;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 0.9rem;
    border-radius: 1.1rem;
    border: 1px solid var(--haarlinie-stark);
    background-color: hsl(var(--card) / 0.55);
    transition: border-color 0.2s, transform 0.2s;
  }
  .heute-karte:hover {
    border-color: color-mix(in srgb, var(--ton) 45%, transparent);
  }
  .heute-karte:active {
    transform: scale(0.98);
  }
  @media (min-width: 1024px) {
    .heute-karte {
      min-width: 0;
    }
  }
  :global([data-oberflaeche='glas']) .heute-karte {
    border-color: var(--glas-rand);
    background-color: transparent;
    background-image: var(--glas-flaeche);
    box-shadow: var(--glas-schatten), var(--glas-kante);
  }
  .heute-label {
    display: block;
    font-size: 11px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: hsl(var(--muted-foreground));
  }
  .heute-wert {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.9rem;
    font-weight: 600;
  }
</style>
