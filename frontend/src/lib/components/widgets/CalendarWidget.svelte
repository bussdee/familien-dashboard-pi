<script lang="ts">
  import {
    CalendarDays, FileText, Lock, Pencil, Plus, Repeat, Trash2, X,
  } from 'lucide-svelte';
  import { differenceInCalendarDays, format, isToday, isTomorrow, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { ApiError, calendarApi } from '$lib/api';
  import type { CalendarEvent, EventDraft, EventRepeat } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  let {
    events = [],
    onRefresh,
  }: { events?: CalendarEvent[]; onRefresh: () => Promise<void> } = $props();

  const repeats: { value: EventRepeat; label: string }[] = [
    { value: 'none', label: 'Einmalig' },
    { value: 'weekly', label: 'Jede Woche' },
    { value: 'monthly', label: 'Jeden Monat' },
    { value: 'yearly', label: 'Jedes Jahr' },
  ];

  const colors = ['#0d9488', '#3b82f6', '#ec4899', '#f59e0b', '#8b5cf6', '#ef4444'];

  function emptyDraft(): EventDraft {
    return {
      title: '',
      description: '',
      location: '',
      date: format(new Date(), 'yyyy-MM-dd'),
      start_time: '17:00',
      end_time: '18:00',
      all_day: false,
      repeat: 'none',
      color: colors[0],
    };
  }

  let showForm = $state(false);
  let editingId = $state<number | null>(null);
  /** Tapped event; opens the actions, which is how this works on a phone. */
  let selectedId = $state<string | null>(null);
  let draft = $state<EventDraft>(emptyDraft());
  let error = $state('');
  let busy = $state(false);

  type Group = { label: string; events: CalendarEvent[] };

  // Group by calendar day so the list reads like a diary rather than a dump.
  const groups = $derived.by<Group[]>(() => {
    const byDay = new Map<string, CalendarEvent[]>();
    for (const event of events) {
      const key = format(parseISO(event.start), 'yyyy-MM-dd');
      const bucket = byDay.get(key);
      if (bucket) bucket.push(event);
      else byDay.set(key, [event]);
    }

    return [...byDay.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .slice(0, 8)
      .map(([key, list]) => {
        const date = parseISO(key);
        const label = isToday(date)
          ? 'Heute'
          : isTomorrow(date)
            ? 'Morgen'
            : format(date, 'EEEE, d. MMMM', { locale: de });
        return { label, events: list };
      });
  });

  function timeLabel(event: CalendarEvent): string {
    if (event.all_day) return 'Ganztägig';
    const start = parseISO(event.start);
    const end = parseISO(event.end);
    return differenceInCalendarDays(end, start) === 0
      ? `${format(start, 'HH:mm')}–${format(end, 'HH:mm')}`
      : format(start, 'HH:mm');
  }

  function startNew() {
    editingId = null;
    draft = emptyDraft();
    error = '';
    showForm = true;
  }

  function select(event: CalendarEvent) {
    selectedId = selectedId === event.id ? null : event.id;
  }

  function startEdit(event: CalendarEvent) {
    if (!event.editable || !event.event_id) return;
    selectedId = null;
    const start = parseISO(event.start);
    const end = parseISO(event.end);
    editingId = event.event_id;
    draft = {
      title: event.title,
      description: event.description,
      location: event.location,
      date: format(start, 'yyyy-MM-dd'),
      start_time: format(start, 'HH:mm'),
      end_time: format(end, 'HH:mm'),
      all_day: event.all_day,
      repeat: (event.repeat as EventRepeat) ?? 'none',
      color: event.color,
    };
    error = '';
    showForm = true;
  }

  async function save(submitEvent: SubmitEvent) {
    submitEvent.preventDefault();
    if (!draft.title.trim() || busy) return;
    busy = true;
    error = '';
    try {
      if (editingId !== null) await calendarApi.update(editingId, draft);
      else await calendarApi.create(draft);
      showForm = false;
      editingId = null;
      selectedId = null;
      await onRefresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Termin konnte nicht gespeichert werden';
    } finally {
      busy = false;
    }
  }

  async function remove(event: CalendarEvent) {
    if (!event.event_id) return;
    const ok = await confirmAction({
      title: `„${event.title}“ löschen?`,
      message: event.recurring
        ? 'Alle Wiederholungen dieses Termins werden entfernt.'
        : undefined,
    });
    if (!ok) return;
    try {
      await calendarApi.remove(event.event_id);
      selectedId = null;
      await onRefresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Termin konnte nicht gelöscht werden';
    }
  }
</script>

<section class="card p-5">
  <header class="mb-4 flex items-center justify-between">
    <div>
      <h2 class="text-lg font-semibold">Kalender</h2>
      <p class="text-sm text-muted-foreground">
        {events.length === 0 ? 'Keine Termine' : `${events.length} Termine`}
      </p>
    </div>
    <button
      class="btn-primary px-3"
      onclick={() => (showForm ? (showForm = false) : startNew())}
      aria-label="Termin hinzufügen"
    >
      {#if showForm}<X class="h-5 w-5" />{:else}<Plus class="h-5 w-5" />{/if}
    </button>
  </header>

  {#if error}
    <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
  {/if}

  {#if showForm}
    <form class="mb-4 space-y-2 rounded-lg border border-border p-3" onsubmit={save}>
      <input class="input" placeholder="Was steht an?" bind:value={draft.title} maxlength="120" />

      <div class="grid grid-cols-2 gap-2">
        <label class="text-xs text-muted-foreground">
          Datum
          <input class="input mt-1" type="date" bind:value={draft.date} />
        </label>
        <label class="text-xs text-muted-foreground">
          Wiederholung
          <select class="input mt-1" bind:value={draft.repeat}>
            {#each repeats as option}<option value={option.value}>{option.label}</option>{/each}
          </select>
        </label>
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input type="checkbox" class="h-4 w-4 rounded" bind:checked={draft.all_day} />
        Ganztägig
      </label>

      {#if !draft.all_day}
        <div class="grid grid-cols-2 gap-2">
          <label class="text-xs text-muted-foreground">
            Von
            <input class="input mt-1" type="time" bind:value={draft.start_time} />
          </label>
          <label class="text-xs text-muted-foreground">
            Bis
            <input class="input mt-1" type="time" bind:value={draft.end_time} />
          </label>
        </div>
      {/if}

      <input class="input" placeholder="Ort (optional)" bind:value={draft.location} maxlength="120" />
      <input
        class="input"
        placeholder="Notiz (optional)"
        bind:value={draft.description}
        maxlength="300"
      />

      <div class="text-xs text-muted-foreground">
        Farbe
        <div class="mt-1 flex flex-wrap gap-2">
          {#each colors as color}
            <button
              type="button"
              class="h-7 w-7 rounded-full transition-transform {draft.color === color
                ? 'scale-110 ring-2 ring-offset-2 ring-offset-card'
                : ''}"
              style="background-color: {color}; --tw-ring-color: {color}"
              aria-label="Farbe {color}"
              onclick={() => (draft.color = color)}
            ></button>
          {/each}
        </div>
      </div>

      <button class="btn-primary w-full" disabled={busy || !draft.title.trim()}>
        {editingId !== null ? 'Änderungen speichern' : 'Termin eintragen'}
      </button>
    </form>
  {/if}

  {#if groups.length === 0}
    <div class="py-8 text-center text-muted-foreground">
      <CalendarDays class="mx-auto mb-2 h-10 w-10 opacity-40" />
      <p class="text-sm">Keine Termine in nächster Zeit</p>
      <p class="mt-1 text-xs">
        Mit <strong>+</strong> eintragen – oder eine <code>.ics</code>-Datei in
        <code>data/ics/</code> ablegen
      </p>
    </div>
  {:else}
    <div class="scrollbar-thin max-h-[340px] space-y-4 overflow-y-auto pr-1">
      {#each groups as group (group.label)}
        <div>
          <h3 class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {group.label}
          </h3>
          <div class="space-y-2">
            {#each group.events as event (event.id)}
              {@const open = selectedId === event.id}
              <article
                class="rounded-xl border-l-4 bg-muted/30 transition-colors {open
                  ? 'bg-accent/60'
                  : ''}"
                style="border-left-color: {event.color}"
              >
                <button
                  class="flex w-full gap-3 p-3 text-left"
                  onclick={() => select(event)}
                  aria-expanded={open}
                >
                  <span class="w-16 shrink-0 text-xs text-muted-foreground">
                    {timeLabel(event)}
                  </span>

                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-1.5 truncate text-sm font-medium">
                      {event.title}
                      {#if event.recurring}
                        <Repeat class="h-3 w-3 shrink-0 text-muted-foreground" />
                      {/if}
                      {#if !event.editable}
                        <Lock class="h-3 w-3 shrink-0 text-muted-foreground" />
                      {/if}
                    </span>
                    {#if event.location}
                      <span class="mt-0.5 block truncate text-xs text-muted-foreground">
                        📍 {event.location}
                      </span>
                    {/if}
                    {#if event.description}
                      <span class="mt-0.5 block truncate text-xs text-muted-foreground">
                        {event.description}
                      </span>
                    {/if}
                  </span>
                </button>

                {#if open}
                  <div class="border-t border-border/60 px-3 py-2">
                    {#if event.editable}
                      <div class="flex gap-2">
                        <button class="btn-outline flex-1 text-sm" onclick={() => startEdit(event)}>
                          <Pencil class="h-4 w-4" /> Bearbeiten
                        </button>
                        <button
                          class="btn-outline flex-1 text-sm text-destructive"
                          onclick={() => remove(event)}
                        >
                          <Trash2 class="h-4 w-4" /> Löschen
                        </button>
                      </div>
                    {:else}
                      <!--
                        Events parsed from an .ics file belong to the app that
                        exported them; editing here would be lost on the next
                        export, so we say where the entry comes from instead.
                      -->
                      <p class="flex items-start gap-2 text-xs text-muted-foreground">
                        <FileText class="mt-0.5 h-3.5 w-3.5 shrink-0" />
                        <span>
                          Kommt aus der Kalenderdatei
                          <strong>{event.calendar}.ics</strong> und lässt sich hier nicht ändern.
                          Ändere ihn in der App, aus der du exportiert hast — oder lege ihn
                          mit <strong>+</strong> als eigenen Termin neu an.
                        </span>
                      </p>
                    {/if}
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
