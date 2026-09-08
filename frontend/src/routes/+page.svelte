<script lang="ts">
  import { onMount } from 'svelte';
  import { RefreshCw, TriangleAlert } from 'lucide-svelte';
  import {
    adminApi, calendarApi, choresApi, connectShoppingSocket,
    devicesApi, notesApi, shoppingApi, weatherApi,
  } from '$lib/api';
  import { connection, session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { layout } from '$lib/stores/layout.svelte';
  import WeatherChip from '$lib/components/WeatherChip.svelte';
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

  onMount(() => {
    void loadAll();
    if (!layout.loaded) void layout.load();

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
      connection.setLive(false);
    };
  });
</script>

<svelte:head><title>Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-7xl px-3 py-4 sm:px-5 sm:py-6">
  <!-- Greeting + the two numbers that decide whether you need to do anything -->
  <header class="mb-5 flex flex-wrap items-start justify-between gap-3">
    <div class="min-w-0">
      <h1 class="text-2xl font-semibold sm:text-3xl">
        {greeting}{$session.user ? `, ${$session.user.name}` : ''}
        <span class="ml-1">{$session.user?.avatar_emoji ?? ''}</span>
      </h1>
      <div class="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1">
        <p class="text-sm capitalize text-muted-foreground">{today}</p>
        <WeatherChip {weather} />
      </div>
    </div>

    <div class="flex items-center gap-2">
      {#if !loading}
        <div class="flex gap-2">
          <a
            href="#chores"
            class="rounded-xl bg-muted px-3 py-2 text-center transition-colors hover:bg-accent"
          >
            <span class="block text-lg font-bold tabular-nums {openTasks > 0 ? 'text-primary' : ''}">
              {openTasks}
            </span>
            <span class="block text-[11px] text-muted-foreground">Aufgaben</span>
          </a>
          <a
            href="#shopping"
            class="rounded-xl bg-muted px-3 py-2 text-center transition-colors hover:bg-accent"
          >
            <span class="block text-lg font-bold tabular-nums">{openItems}</span>
            <span class="block text-[11px] text-muted-foreground">Einkauf</span>
          </a>
          {#if me}
            <a
              href="/rangliste"
              class="rounded-xl bg-muted px-3 py-2 text-center transition-colors hover:bg-accent"
            >
              <span class="block text-lg font-bold tabular-nums text-amber-500">
                {me.total_points}
              </span>
              <span class="block text-[11px] text-muted-foreground">Punkte</span>
            </a>
          {/if}
        </div>
      {/if}
      <button
        class="btn-ghost rounded-xl px-2"
        onclick={manualRefresh}
        disabled={refreshing}
        aria-label="Alles aktualisieren"
        title="Alles aktualisieren"
      >
        <RefreshCw class="h-5 w-5 {refreshing ? 'animate-spin' : ''}" />
      </button>
    </div>
  </header>

  {#if $session.user?.pin_is_default}
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
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {#each Array(6) as _, i (i)}
        <div class="card h-56 animate-pulse bg-muted/40"></div>
      {/each}
    </div>
  {:else}
    <!--
      Order and visibility come from the person's own layout preference; the
      default puts the two lists people act on first.
    -->
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {#each layout.visible as widget (widget.id)}
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
