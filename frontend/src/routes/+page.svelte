<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { RefreshCw, TriangleAlert } from 'lucide-svelte';
  import {
    adminApi, calendarApi, choresApi, connectShoppingSocket,
    devicesApi, notesApi, shoppingApi, weatherApi,
  } from '$lib/api';
  import { connection, session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { layout } from '$lib/stores/layout.svelte';
  import WeatherHero from '$lib/components/WeatherHero.svelte';
  import PointsBand from '$lib/components/PointsBand.svelte';
  import CalendarWidget from '$lib/components/widgets/CalendarWidget.svelte';
  import ShoppingWidget from '$lib/components/widgets/ShoppingWidget.svelte';
  import ChoresWidget from '$lib/components/widgets/ChoresWidget.svelte';
  import NotesWidget from '$lib/components/widgets/NotesWidget.svelte';
  import DevicesWidget from '$lib/components/widgets/DevicesWidget.svelte';
  import CountdownWidget from '$lib/components/widgets/CountdownWidget.svelte';
  import PhotoWidget from '$lib/components/widgets/PhotoWidget.svelte';
  import LinksWidget from '$lib/components/widgets/LinksWidget.svelte';
  import { LayoutGrid } from 'lucide-svelte';
  import type {
    CalendarEvent, Chore, DeviceStatus, Note, ShoppingItem, User, WeatherData,
  } from '$lib/types';

  let weather = $state<WeatherData | null>(null);
  let events = $state<CalendarEvent[]>([]);
  let shopping = $state<ShoppingItem[]>([]);
  let notes = $state<Note[]>([]);
  let chores = $state<Chore[]>([]);
  let devices = $state<DeviceStatus[]>([]);
  let users = $state<User[]>([]);

  let loading = $state(true);
  let refreshing = $state(false);
  let failures = $state<string[]>([]);

  const greeting = $derived.by(() => {
    const hour = new Date().getHours();
    if (hour < 5) return 'Gute Nacht';
    if (hour < 11) return 'Guten Morgen';
    if (hour < 18) return 'Hallo';
    return 'Guten Abend';
  });

  const today = $derived(
    new Date().toLocaleDateString('de-DE', { weekday: 'long', day: 'numeric', month: 'long' }),
  );

  const openTasks = $derived(chores.filter((c) => c.is_overdue || c.days_until_due <= 0).length);
  const openItems = $derived(shopping.filter((i) => !i.checked).length);
  const me = $derived(board.for($session.user?.id));

  /**
   * Every widget loads independently — one failing endpoint (no internet for
   * the weather, say) must not blank the whole dashboard.
   */
  async function loadAll() {
    const problems: string[] = [];
    const run = async <T,>(label: string, fn: () => Promise<T>, apply: (value: T) => void) => {
      try {
        apply(await fn());
      } catch {
        problems.push(label);
      }
    };

    await Promise.all([
      run('Wetter', () => weatherApi.get(), (v) => (weather = v)),
      run('Kalender', () => calendarApi.events(45), (v) => (events = v.events)),
      run('Einkaufsliste', () => shoppingApi.list(), (v) => (shopping = v)),
      run('Notizen', () => notesApi.list(), (v) => (notes = v)),
      run('Aufgaben', () => choresApi.list(), (v) => (chores = v)),
      run('Geräte', () => devicesApi.list(), (v) => (devices = v)),
      run('Rangliste', () => board.refresh(), () => {}),
    ]);

    failures = problems;
    connection.synced();
    loading = false;
  }

  async function manualRefresh() {
    refreshing = true;
    try {
      await loadAll();
    } finally {
      refreshing = false;
    }
  }

  async function reloadCalendar() {
    events = (await calendarApi.events(45)).events;
  }

  async function reloadChores() {
    chores = await choresApi.list();
  }

  async function reloadDevices() {
    devices = await devicesApi.list();
  }

  /**
   * Ein Wandgerät soll nicht stundenlang eine Aufgabenliste anstarren. Nach
   * einer Weile ohne Berührung wird daraus ein Bilderrahmen; eine Berührung
   * dort führt zurück in den Familien-Modus, nie in ein fremdes Konto.
   */
  const RUHE_MS = 5 * 60_000;
  let ruheTimer: ReturnType<typeof setTimeout> | null = null;

  function ruheNeuStarten() {
    if (ruheTimer) clearTimeout(ruheTimer);
    if (!$session.device) return;
    ruheTimer = setTimeout(() => void goto('/diashow'), RUHE_MS);
  }

  onMount(() => {
    void loadAll();
    if (!layout.loaded) void layout.load();
    ruheNeuStarten();

    if ($session.user?.role === 'admin') {
      adminApi.listUsers().then((list) => (users = list)).catch(() => {});
    }

    const disconnect = connectShoppingSocket(
      (event) => {
        if (event.action === 'cleared') {
          shopping = shopping.filter((i) => !i.checked);
          return;
        }
        if (event.action === 'deleted') {
          shopping = shopping.filter((i) => i.id !== event.item.id);
          return;
        }
        const known = shopping.some((i) => i.id === event.item.id);
        shopping = known
          ? shopping.map((i) => (i.id === event.item.id ? event.item : i))
          : [event.item, ...shopping];
      },
      (live) => connection.setLive(live),
    );

    // Keep a wall tablet current without anyone touching it.
    const timer = setInterval(() => void loadAll(), 5 * 60 * 1000);

    return () => {
      disconnect();
      clearInterval(timer);
      if (ruheTimer) clearTimeout(ruheTimer);
      connection.setLive(false);
    };
  });

</script>

<svelte:head><title>Familien Dashboard</title></svelte:head>

<!-- Jede Berührung schiebt den Ruhezustand nach hinten. -->
<svelte:window onpointerdown={ruheNeuStarten} onkeydown={ruheNeuStarten} />

<!-- Volle Breite: auf einem großen Monitor bleibt sonst links und rechts
     Rand stehen, und der Flurbildschirm verschenkt die Hälfte der Fläche. -->
<div class="w-full px-4 py-5 sm:px-7 sm:py-7 2xl:px-10">
  <!-- Begrüßung links, Wetter rechts — beides ohne Kasten -->
  <header class="mb-6 flex flex-wrap items-start justify-between gap-x-10 gap-y-6">
    <div class="min-w-0">
      <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {today}
      </p>
      <h1 class="mt-2 font-display text-[2.5rem] font-light leading-[1.05] tracking-tight sm:text-5xl">
        {#if $session.device}
          <!-- Familien-Modus: das Gerät grüßt niemanden persönlich, weil es
               nicht weiß (und nicht wissen soll), wer gerade davorsteht. -->
          {greeting}<span class="font-medium italic">, Familie</span>
        {:else}
          {greeting}{$session.user ? ',' : ''}
          {#if $session.user}
            <span class="font-medium italic">{$session.user.name}</span>
          {/if}
        {/if}
      </h1>
      {#if !loading}
        <p class="mt-3 text-sm font-light text-muted-foreground">
          <a href="#chores" class="transition-colors hover:text-foreground">
            {openTasks}
            {openTasks === 1 ? 'Aufgabe' : 'Aufgaben'} offen
          </a>
          <span class="mx-2 opacity-40">·</span>
          <a href="#shopping" class="transition-colors hover:text-foreground">
            {openItems} auf der Einkaufsliste
          </a>
        </p>
      {/if}
    </div>

    <div class="min-w-0 flex-1 sm:max-w-2xl">
      <WeatherHero {weather} />
    </div>

    <button
      class="btn-ghost shrink-0 rounded-full px-2 text-muted-foreground"
      onclick={manualRefresh}
      disabled={refreshing}
      aria-label="Alles aktualisieren"
      title="Alles aktualisieren"
    >
      <RefreshCw class="h-5 w-5 {refreshing ? 'animate-spin' : ''}" />
    </button>
  </header>

  {#if !$session.device}
    <PointsBand {me} total={board.scores.length} />
  {/if}

  {#if $session.user?.pin_is_default && !$session.device}
    <a
      href="/settings"
      class="mb-4 flex items-center gap-2 rounded-xl bg-amber-500/10 px-4 py-3 text-sm text-amber-700 transition-colors hover:bg-amber-500/15 dark:text-amber-400"
    >
      <TriangleAlert class="h-4 w-4 shrink-0" />
      Du benutzt noch die Standard-PIN. Jetzt in den Einstellungen ändern →
    </a>
  {/if}

  {#if failures.length > 0}
    <p class="mb-4 flex items-center gap-2 rounded-xl bg-muted px-4 py-2.5 text-sm text-muted-foreground">
      <TriangleAlert class="h-4 w-4 shrink-0" />
      Nicht geladen: {failures.join(', ')}
    </p>
  {/if}

  {#if loading}
    <div class="widget-raster">
      {#each Array(6) as _, i (i)}
        <div class="h-56 animate-pulse rounded-xl bg-muted/30"></div>
      {/each}
    </div>
  {:else}
    <!--
      Order and visibility come from the person's own layout preference; the
      default puts the two lists people act on first.
    -->
    <div class="widget-raster mt-2">
      <!-- Links gehören einer Person. Am Wandgerät bliebe die Kachel leer
           und würde nur Fragen aufwerfen. -->
      {#each layout.visible.filter((w) => !($session.device && w.id === 'links')) as widget (widget.id)}
        <div id={widget.id} class="scroll-mt-20">
          {#if widget.id === 'chores'}
            <ChoresWidget bind:chores {users} onRefresh={reloadChores} />
          {:else if widget.id === 'shopping'}
            <ShoppingWidget bind:items={shopping} />
          {:else if widget.id === 'calendar'}
            <CalendarWidget {events} onRefresh={reloadCalendar} />
          {:else if widget.id === 'links'}
            <LinksWidget />
          {:else if widget.id === 'countdown'}
            <CountdownWidget {events} />
          {:else if widget.id === 'notes'}
            <NotesWidget bind:notes />
          {:else if widget.id === 'photos'}
            <PhotoWidget />
          {:else if widget.id === 'devices'}
            <DevicesWidget {devices} onRefresh={reloadDevices} />
          {/if}
        </div>
      {/each}
    </div>

    {#if layout.visible.length === 0}
      <section class="card p-8 text-center">
        <LayoutGrid class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-40" />
        <p class="font-medium">Alle Fenster ausgeblendet</p>
        <a href="/ansicht" class="btn-primary mt-4 inline-flex">Ansicht anpassen</a>
      </section>
    {:else}
      <a
        href="/ansicht"
        class="mt-5 flex items-center justify-center gap-2 rounded-xl py-3 text-sm text-muted-foreground transition-colors hover:bg-accent"
      >
        <LayoutGrid class="h-4 w-4" />
        Fenster anordnen oder ausblenden
      </a>
    {/if}
  {/if}
</div>
