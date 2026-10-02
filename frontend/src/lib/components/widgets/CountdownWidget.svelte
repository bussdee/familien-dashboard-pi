<script lang="ts">
  import { t, dfLocale } from '$lib/i18n';
  import { untrack } from 'svelte';
  import {
    Cake, CalendarHeart, FileText, Pencil, PartyPopper, Plane, Timer, Trash2,
  } from 'lucide-svelte';
  import { differenceInCalendarDays, format, parseISO, startOfDay } from 'date-fns';
  import { ApiError, calendarApi } from '$lib/api';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { schnell } from '$lib/stores/schnell.svelte';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
  import type { CalendarEvent } from '$lib/types';

  /**
   * Die Übersicht reicht dem Countdown ihre Termine der nächsten Wochen
   * herein. Das genügt für „Ferien in drei Wochen", aber nicht für die Frage,
   * die Kinder wirklich stellen: wie viele Tage noch bis zum Geburtstag. Der
   * liegt fast immer Monate entfernt und stand deshalb nie in der Liste.
   *
   * Also holt sich diese Kachel ihr eigenes, weites Fenster. Die
   * übergebenen Termine sind nur der Anfang, damit sofort etwas dasteht.
   */
  const FENSTER_TAGE = 400;

  let {
    events = [],
    onRefresh,
  }: { events?: CalendarEvent[]; onRefresh?: () => Promise<void> } = $props();

  let weit = $state<CalendarEvent[] | null>(null);
  const quelle = $derived(weit ?? events);

  /** Angetippte Zeile: klappt Bearbeiten und Löschen auf, wie im Kalender. */
  let offenId = $state<string | null>(null);
  let fehler = $state('');

  async function ladeWeit() {
    try {
      weit = (await calendarApi.events(FENSTER_TAGE)).events;
    } catch {
      // Bleibt bei den übergebenen Terminen — lieber die nächsten Wochen
      // als eine leere Kachel.
    }
  }

  // Beim Öffnen und immer dann, wenn die Übersicht ihre Termine neu geladen
  // hat — etwa weil im Kalender ein Termin geändert wurde. Sonst zeigte diese
  // Kachel danach noch den alten Namen.
  $effect(() => {
    events;
    untrack(() => void ladeWeit());
  });

  function bearbeiten(event: CalendarEvent) {
    offenId = null;
    schnell.terminBearbeiten = event;
  }

  async function loeschen(event: CalendarEvent) {
    if (!event.event_id) return;
    const ok = await confirmAction({
      title: t('„{0}“ löschen?', [event.title]),
      message: event.recurring
        ? t('Der Termin verschwindet auch aus dem Kalender — samt allen Wiederholungen.')
        : t('Der Termin verschwindet auch aus dem Kalender.'),
    });
    if (!ok) return;
    try {
      await calendarApi.remove(event.event_id);
      offenId = null;
      fehler = '';
      await Promise.all([ladeWeit(), onRefresh?.()]);
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : t('Termin konnte nicht gelöscht werden');
    }
  }

  // Keywords that turn an ordinary calendar entry into a countdown.
  const kinds = [
    // Suchwörter in den Termintiteln — deutsch und englisch, unabhängig von
    // der Sprache der Oberfläche: Ein Termin heisst, wie man ihn eingetragen hat.
    { match: ['geburtstag', 'birthday', 'geburi'], icon: Cake, color: '#ec4899' },
    { match: ['ferien', 'urlaub', 'reise', 'flug', 'holiday', 'vacation', 'trip', 'flight'], icon: Plane, color: '#f59e0b' },
    { match: ['jubiläum', 'hochzeit', 'jahrestag', 'anniversary', 'wedding'], icon: CalendarHeart, color: '#ef4444' },
    { match: ['feier', 'party', 'fest', 'weihnachten', 'ostern', 'silvester', 'christmas', 'easter', 'celebration'], icon: PartyPopper, color: '#8b5cf6' },
  ];

  const countdowns = $derived.by(() => {
    const today = startOfDay(new Date());
    return quelle
      .map((event) => {
        const title = event.title.toLowerCase();
        const kind = kinds.find((k) => k.match.some((m) => title.includes(m)));
        if (!kind) return null;
        const days = differenceInCalendarDays(parseISO(event.start), today);
        if (days < 0) return null;
        return { event, days, icon: kind.icon, color: kind.color };
      })
      .filter((x): x is NonNullable<typeof x> => x !== null)
      // Ein jährlicher Termin kommt im weiten Fenster zweimal vor — einmal
      // dieses Jahr, einmal nächstes. Nur der nächste zählt.
      .filter((item, i, alle) => alle.findIndex((x) => x.event.title === item.event.title) === i)
      .sort((a, b) => a.days - b.days)
      .slice(0, 5);
  });

  const label = (days: number) =>
    days === 0 ? t('Heute!') : days === 1 ? t('Morgen') : t('in {0} Tagen', [days]);

  // Bei einem Geburtstag in acht Monaten ist die Tageszahl allein unhandlich.
  const dazu = (days: number) => {
    if (days < 31) return '';
    const monate = Math.round(days / 30.44);
    return monate === 1 ? t('gut ein Monat') : t('gut {0} Monate', [monate]);
  };
</script>

{#snippet zeile()}
  {countdowns.length === 0 ? t('Worauf wir uns freuen') : t('{0} in Sicht', [countdowns.length])}
{/snippet}

<Kachel ton="var(--ton-countdown)" titel={t('Countdowns')} icon={Timer} {zeile}>
  {#if countdowns.length === 0}
    <KachelLeer
      icon={PartyPopper}
      titel={t('Keine Countdowns')}
      hinweis={t('Termine mit „Geburtstag“, „Ferien“ oder „Feier“ erscheinen hier von selbst.')}
    />
  {:else}
    {#if fehler}
      <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{fehler}</p>
    {/if}
    <div class="space-y-2">
      {#each countdowns as item (item.event.id)}
        {@const Icon = item.icon}
        {@const auf = offenId === item.event.id}
        <div class="rounded-lg transition-colors {auf ? 'bg-muted/25' : ''}">
        <button
          class="flex w-full items-center gap-3 rounded-lg px-1 py-2.5 text-left"
          onclick={() => (offenId = auf ? null : item.event.id)}
          aria-expanded={auf}
          aria-label={t('{0} – Optionen', [item.event.title])}
        >
          <div
            class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl"
            style="background-color: {item.color}1a"
          >
            <Icon class="h-5 w-5" style="color: {item.color}" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{item.event.title}</p>
            <p class="text-xs text-muted-foreground">
              {format(parseISO(item.event.start), t('EEEE, d. MMMM'), { locale: dfLocale })}
            </p>
          </div>
          <div class="shrink-0 text-right">
            <p class="text-xl font-bold tabular-nums" style="color: {item.color}">
              {item.days}
            </p>
            <p class="text-[11px] text-muted-foreground">{label(item.days)}</p>
            {#if dazu(item.days)}
              <p class="text-[10px] text-muted-foreground opacity-70">{dazu(item.days)}</p>
            {/if}
          </div>
        </button>

        {#if auf}
          <div class="border-t border-border/60 px-2 py-2">
            {#if item.event.editable && item.event.event_id}
              <div class="flex gap-2">
                <button class="btn-outline flex-1 text-sm" onclick={() => bearbeiten(item.event)}>
                  <Pencil class="h-4 w-4" /> {t('Bearbeiten')}
                </button>
                <button
                  class="btn-outline flex-1 text-sm text-destructive"
                  onclick={() => loeschen(item.event)}
                >
                  <Trash2 class="h-4 w-4" /> {t('Löschen')}
                </button>
              </div>
            {:else}
              <!-- Aus einer .ics-Datei: gehört der App, die sie exportiert
                   hat. Ein Knopf, der nichts bewirkt, wäre schlimmer als
                   der Hinweis, woran es liegt. -->
              <p class="flex items-start gap-2 text-xs text-muted-foreground">
                <FileText class="mt-0.5 h-3.5 w-3.5 shrink-0" />
                <span>
                  {t('Kommt aus der Kalenderdatei')} <strong>{item.event.calendar}.ics</strong> {t('und lässt sich hier nicht ändern.')}
                </span>
              </p>
            {/if}
          </div>
        {/if}
        </div>
      {/each}
    </div>
  {/if}
</Kachel>
