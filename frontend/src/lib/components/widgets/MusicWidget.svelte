<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ChevronRight, CornerLeftUp, Folder, Music, Play, RefreshCw, Search, TriangleAlert, X,
  } from 'lucide-svelte';
  import { ApiError, musicApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { player, titelVon, zeit } from '$lib/stores/player.svelte';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
  import type { MusicBrowse, MusicStatus, Track } from '$lib/types';

  let status = $state<MusicStatus | null>(null);
  let inhalt = $state<MusicBrowse | null>(null);
  let loading = $state(true);
  let fehler = $state('');

  let suche = $state('');
  let treffer = $state<Track[] | null>(null);
  let sucheLaeuft = $state(false);
  let suchTimer: ReturnType<typeof setTimeout> | null = null;

  const istAdmin = $derived($session.user?.role === 'admin');
  const laeuftEinlesen = $derived(status?.progress?.running === true);

  /** Die Brotkrumen: Wurzel, dann jeder Ordner auf dem Weg hierher. */
  const pfadTeile = $derived.by(() => {
    const pfad = inhalt?.path ?? '';
    if (!pfad) return [];
    const teile = pfad.split('/');
    return teile.map((name, i) => ({ name, path: teile.slice(0, i + 1).join('/') }));
  });

  async function ladeStatus() {
    try {
      status = await musicApi.status();
    } catch {
      status = null;
    }
  }

  async function oeffne(pfad: string) {
    fehler = '';
    treffer = null;
    suche = '';
    try {
      inhalt = await musicApi.browse(pfad);
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Ordner konnte nicht geladen werden';
    }
  }

  function sucheGeaendert() {
    if (suchTimer) clearTimeout(suchTimer);
    const q = suche.trim();
    if (q.length < 2) {
      treffer = null;
      sucheLaeuft = false;
      return;
    }
    // Getippt wird schneller, als eine Abfrage über 20 000 Zeilen zurückkommt.
    sucheLaeuft = true;
    suchTimer = setTimeout(async () => {
      try {
        treffer = (await musicApi.search(q)).tracks;
      } catch {
        treffer = [];
      } finally {
        sucheLaeuft = false;
      }
    }, 300);
  }

  /**
   * Ein Titel startet immer seine ganze Liste ab dieser Stelle — bei einem
   * Hörspiel läuft Teil 2 danach von selbst weiter.
   */
  function spiele(liste: Track[], i: number, quelle: string) {
    player.spiele(liste, i, quelle);
  }

  async function neuEinlesen() {
    fehler = '';
    try {
      await musicApi.rescan();
      await ladeStatus();
      // Beim ersten Durchlauf über eine grosse Sammlung dauert es Minuten.
      // Solange nachfragen, bis er durch ist, dann den Ordner neu holen.
      const timer = setInterval(async () => {
        await ladeStatus();
        if (!status?.progress?.running) {
          clearInterval(timer);
          await oeffne(inhalt?.path ?? '');
        }
      }, 3000);
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Einlesen fehlgeschlagen';
    }
  }

  onMount(() => {
    void (async () => {
      await ladeStatus();
      if (status?.enabled) await oeffne('');
      loading = false;
    })();

    // Läuft gerade ein Durchlauf, wächst die Zahl der Titel im Hintergrund.
    const timer = setInterval(() => {
      if (status?.progress?.running) void ladeStatus();
    }, 4000);
    return () => {
      clearInterval(timer);
      if (suchTimer) clearTimeout(suchTimer);
    };
  });
</script>

{#snippet zeile()}
  {#if loading}
    Lade…
  {:else if !status?.enabled}
    Nicht eingerichtet
  {:else if laeuftEinlesen}
    Lese ein… {status.progress.scanned.toLocaleString('de-DE')} Dateien
  {:else if status.tracks === 0}
    Noch keine Musik gefunden
  {:else}
    {status.tracks.toLocaleString('de-DE')} Titel
  {/if}
{/snippet}

{#snippet aktionen()}
  {#if istAdmin && status?.enabled && status.available}
    <button
      class="btn-ghost rounded-full px-2 text-muted-foreground"
      onclick={neuEinlesen}
      disabled={laeuftEinlesen}
      aria-label="Musikordner neu einlesen"
      title="Musikordner neu einlesen"
    >
      <RefreshCw class="h-4 w-4 {laeuftEinlesen ? 'animate-spin' : ''}" />
    </button>
  {/if}
{/snippet}

<Kachel
  titel="Musik"
  icon={Music}
  {zeile}
  {aktionen}
  randlos
  {fehler}
  onFehlerZu={() => (fehler = '')}
>
  <div class="px-5 pb-5">

    {#if loading}
      <div class="space-y-2">
        {#each Array(4) as _, i (i)}
          <div class="h-10 animate-pulse rounded-lg bg-muted/30"></div>
        {/each}
      </div>
    {:else if !status?.enabled}
      <KachelLeer
        icon={Music}
        titel="Kein Musikordner eingerichtet"
        hinweis={istAdmin ? '' : 'Ein Elternteil richtet das in der Verwaltung ein.'}
      >
        {#snippet aktion()}
          {#if istAdmin}
            <a href="/admin" class="btn-outline text-sm">In der Verwaltung einrichten</a>
          {/if}
        {/snippet}
      </KachelLeer>
    {:else if !status.available}
      <!-- Eine externe Platte kann abgemeldet sein. Das ist kein Fehler,
           sondern ein Zustand — und der gehört gesagt. -->
      <p
        class="flex items-start gap-2 rounded-xl bg-amber-500/10 px-3 py-3 text-sm text-amber-700 dark:text-amber-400"
      >
        <TriangleAlert class="mt-0.5 h-4 w-4 shrink-0" />
        <span>
          Der Musikordner ist gerade nicht erreichbar. Ist die Festplatte
          angeschlossen? Der Index bleibt so lange stehen.
        </span>
      </p>
    {:else if status.tracks === 0 && !laeuftEinlesen}
      <KachelLeer
        icon={Music}
        titel="Hier ist noch keine Musik"
        hinweis={istAdmin
          ? status.subdir
            ? `Im Ordner „${status.subdir}" liegt nichts Hörbares.`
            : 'Im eingehängten Ordner liegt nichts Hörbares.'
          : 'Ein Elternteil wählt in der Verwaltung den richtigen Ordner.'}
      >
        {#snippet aktion()}
          {#if istAdmin}
            <a href="/admin" class="btn-outline text-sm">Anderen Ordner wählen</a>
          {/if}
        {/snippet}
      </KachelLeer>
    {:else}
      <!-- Suchen: Bei 20 000 Dateien kommt man durch Blättern allein nicht
           ans Ziel, wenn man schon weiss, was man hören will. -->
      <div class="relative mb-3">
        <Search
          class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
        />
        <input
          class="input pl-9"
          placeholder="Titel, Interpret oder Album suchen…"
          bind:value={suche}
          oninput={sucheGeaendert}
          autocapitalize="off"
          spellcheck="false"
        />
        {#if suche}
          <button
            class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted-foreground"
            onclick={() => {
              suche = '';
              sucheGeaendert();
            }}
            aria-label="Suche leeren"
          >
            <X class="h-4 w-4" />
          </button>
        {/if}
      </div>

      {#if treffer !== null}
        {#if sucheLaeuft}
          <p class="py-6 text-center text-sm text-muted-foreground">Suche…</p>
        {:else if treffer.length === 0}
          <p class="py-6 text-center text-sm text-muted-foreground">
            Nichts gefunden für „{suche}".
          </p>
        {:else}
          <p class="mb-2 text-xs text-muted-foreground">
            {treffer.length}{treffer.length === 200 ? '+' : ''} Treffer
          </p>
          <ul class="scrollbar-thin max-h-72 space-y-1 overflow-y-auto pr-1">
            {#each treffer as track, i (track.id)}
              <li>
                <button
                  class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left transition-colors hover:bg-accent
                    {player.current?.id === track.id ? 'bg-primary/10 text-primary' : ''}"
                  onclick={() => spiele(treffer ?? [], i, `Suche: ${suche}`)}
                >
                  <Play class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span class="min-w-0 flex-1">
                    <span class="block truncate text-sm">{titelVon(track)}</span>
                    <span class="block truncate text-[11px] text-muted-foreground">
                      {track.folder || 'Wurzel'}
                    </span>
                  </span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {:else if inhalt}
        <!-- Brotkrumen -->
        <div class="mb-2 flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
          <button class="rounded px-1.5 py-0.5 hover:bg-accent" onclick={() => oeffne('')}>
            Alle
          </button>
          {#each pfadTeile as teil (teil.path)}
            <ChevronRight class="h-3 w-3 shrink-0 opacity-50" />
            <button
              class="max-w-[10rem] truncate rounded px-1.5 py-0.5 hover:bg-accent"
              onclick={() => oeffne(teil.path)}
            >
              {teil.name}
            </button>
          {/each}
        </div>

        <ul class="scrollbar-thin max-h-72 space-y-1 overflow-y-auto pr-1">
          {#if inhalt.path}
            <li>
              <button
                class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent"
                onclick={() => oeffne(inhalt?.parent ?? '')}
              >
                <CornerLeftUp class="h-4 w-4 shrink-0" /> Eine Ebene höher
              </button>
            </li>
          {/if}

          {#each inhalt.folders as ordner (ordner.path)}
            <li>
              <button
                class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left transition-colors hover:bg-accent"
                onclick={() => oeffne(ordner.path)}
              >
                <Folder class="h-4 w-4 shrink-0 text-muted-foreground" />
                <span class="min-w-0 flex-1 truncate text-sm font-medium">{ordner.name}</span>
                <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
                  {ordner.tracks.toLocaleString('de-DE')}
                </span>
              </button>
            </li>
          {/each}

          {#each inhalt.tracks as track, i (track.id)}
            <li>
              <button
                class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left transition-colors hover:bg-accent
                  {player.current?.id === track.id ? 'bg-primary/10 text-primary' : ''}"
                onclick={() =>
                  spiele(inhalt?.tracks ?? [], i, inhalt?.path || 'Musik')}
              >
                <Play class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm">{titelVon(track)}</span>
                  {#if track.artist}
                    <span class="block truncate text-[11px] text-muted-foreground">
                      {track.artist}
                    </span>
                  {/if}
                </span>
                {#if track.duration > 0}
                  <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
                    {zeit(track.duration)}
                  </span>
                {/if}
              </button>
            </li>
          {/each}

          {#if inhalt.folders.length === 0 && inhalt.tracks.length === 0}
            <li class="py-6 text-center text-sm text-muted-foreground">
              {#if laeuftEinlesen}
                Der Ordner wird gerade eingelesen…
              {:else}
                Dieser Ordner ist leer.
              {/if}
            </li>
          {/if}
        </ul>
      {/if}
    {/if}
  </div>
</Kachel>
