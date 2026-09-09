<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { format, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { ChevronLeft, ChevronRight, Images, Pause, Play, X } from 'lucide-svelte';
  import { calendarApi, choresApi, photosApi, shoppingApi, weatherApi } from '$lib/api';
  import { diashow } from '$lib/stores/diashow.svelte';
  import type { CalendarEvent, Chore, Photo, WeatherData } from '$lib/types';

  /** Wie lange ein Bild stehen bleibt. Kurz genug, dass es lebendig wirkt. */
  const WECHSEL_MS = 12_000;
  /** Zahlen und Termine altern langsamer als Bilder. */
  const DATEN_MS = 5 * 60_000;

  let photos = $state<Photo[]>([]);
  let index = $state(0);
  let paused = $state(false);
  let bedienung = $state(false);
  let jetzt = $state(new Date());

  let weather = $state<WeatherData | null>(null);
  let chores = $state<Chore[]>([]);
  let events = $state<CalendarEvent[]>([]);
  let offeneEinkaeufe = $state(0);

  let bildTimer: ReturnType<typeof setInterval> | null = null;
  let uhrTimer: ReturnType<typeof setInterval> | null = null;
  let datenTimer: ReturnType<typeof setInterval> | null = null;
  let bedienTimer: ReturnType<typeof setTimeout> | null = null;

  const aktuell = $derived(photos.length > 0 ? photos[index % photos.length] : null);
  const offeneAufgaben = $derived(chores.filter((c) => c.is_due).length);

  const heute = $derived(
    events
      .filter((e) => new Date(e.start).toDateString() === jetzt.toDateString())
      .slice(0, 2),
  );

  const regenSatz = $derived.by(() => {
    const r = weather?.rain;
    if (!r) return null;
    if (r.now) return r.ends_at ? `Regen bis ${uhr(r.ends_at)}` : 'Es regnet';
    if (r.starts_at) return `Regen ab ${uhr(r.starts_at)}`;
    return null;
  });

  const uhr = (iso: string) => format(parseISO(iso), 'HH:mm', { locale: de });

  async function ladeDaten() {
    // Jede Anfrage für sich: fehlt das Wetter, läuft die Show trotzdem weiter.
    const [w, c, e, s] = await Promise.allSettled([
      weatherApi.get(),
      choresApi.list(),
      calendarApi.events(2),
      shoppingApi.list(),
    ]);
    if (w.status === 'fulfilled') weather = w.value;
    if (c.status === 'fulfilled') chores = c.value;
    if (e.status === 'fulfilled') events = e.value.events;
    if (s.status === 'fulfilled') offeneEinkaeufe = s.value.filter((i) => !i.checked).length;
  }

  function weiter(schritt = 1) {
    if (photos.length === 0) return;
    index = (index + schritt + photos.length) % photos.length;
    starteBildwechsel();
  }

  function starteBildwechsel() {
    if (bildTimer) clearInterval(bildTimer);
    if (paused || photos.length < 2) return;
    bildTimer = setInterval(() => (index = (index + 1) % photos.length), WECHSEL_MS);
  }

  /** Die Knöpfe erscheinen bei Berührung und verschwinden von allein wieder. */
  function zeigeBedienung() {
    bedienung = true;
    // Die Abspielleiste im Seitenlayout kann diese Seite nicht sehen. Damit
    // sie mit aus- und einblendet, läuft der Zustand über einen gemeinsamen
    // Speicher.
    diashow.wach = true;
    if (bedienTimer) clearTimeout(bedienTimer);
    bedienTimer = setTimeout(() => {
      bedienung = false;
      diashow.wach = false;
    }, 4000);
  }

  function beenden() {
    goto('/');
  }

  onMount(async () => {
    try {
      photos = (await photosApi.list()).photos;
    } catch {
      photos = [];
    }
    await ladeDaten();
    starteBildwechsel();
    uhrTimer = setInterval(() => (jetzt = new Date()), 10_000);
    datenTimer = setInterval(ladeDaten, DATEN_MS);
  });

  onDestroy(() => {
    if (bildTimer) clearInterval(bildTimer);
    if (uhrTimer) clearInterval(uhrTimer);
    if (datenTimer) clearInterval(datenTimer);
    if (bedienTimer) clearTimeout(bedienTimer);
  });

  $effect(() => {
    paused;
    starteBildwechsel();
  });
</script>

<svelte:head>
  <title>Diashow · Familien Dashboard</title>
</svelte:head>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape') beenden();
    if (e.key === 'ArrowRight') weiter(1);
    if (e.key === 'ArrowLeft') weiter(-1);
    if (e.key === ' ') {
      e.preventDefault();
      paused = !paused;
    }
  }}
/>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-[80] overflow-hidden bg-black text-white"
  onpointermove={zeigeBedienung}
  onpointerdown={zeigeBedienung}
  role="presentation"
>
  {#if aktuell}
    {#key aktuell.name}
      <img
        src={photosApi.url(aktuell.name)}
        alt=""
        class="absolute inset-0 h-full w-full animate-fade-in object-cover"
      />
    {/key}
    <!-- Zwei Verläufe, damit die Schrift über jedem Bild lesbar bleibt -->
    <div
      class="absolute inset-0"
      style="background: linear-gradient(180deg, rgba(0,0,0,.55) 0%, rgba(0,0,0,.05) 30%, rgba(0,0,0,.06) 55%, rgba(0,0,0,.72) 100%)"
    ></div>
  {:else}
    <div class="absolute inset-0 flex flex-col items-center justify-center gap-3 text-white/50">
      <Images class="h-12 w-12" />
      <p class="text-sm">Noch keine Fotos im Rahmen</p>
      <a href="/" class="mt-2 text-sm text-primary hover:underline">Zurück zur Übersicht</a>
    </div>
  {/if}

  <!-- Uhrzeit gehört auf ein Wandtablet, auch wenn niemand hinsieht -->
  <div class="absolute left-8 top-7 sm:left-11 sm:top-9">
    <div class="font-display text-6xl font-light leading-none tracking-tight sm:text-7xl">
      {format(jetzt, 'HH:mm')}
    </div>
    <div class="mt-2 text-sm font-light text-white/70">
      {format(jetzt, 'EEEE, d. MMMM', { locale: de })}
    </div>
  </div>

  {#if weather}
    <div class="absolute right-8 top-7 text-right sm:right-11 sm:top-9">
      <div class="font-display text-4xl font-light leading-none sm:text-5xl">
        {Math.round(weather.current.temperature)}°
      </div>
      {#if regenSatz}
        <div class="mt-1.5 text-sm font-light text-sky-200">{regenSatz}</div>
      {:else}
        <div class="mt-1.5 text-sm font-light text-white/65">{weather.current.description}</div>
      {/if}
    </div>
  {/if}

  <!-- Was heute noch ansteht: der Grund, warum das Tablet dort hängt -->
  <div class="absolute bottom-8 left-8 right-8 sm:bottom-10 sm:left-11 sm:right-11">
    <div class="flex flex-wrap items-end justify-between gap-6">
      <div class="min-w-0">
        <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-white/50">Heute noch</p>
        <div class="mt-3 flex flex-wrap items-center gap-x-7 gap-y-2 text-base font-light sm:text-lg">
          {#if offeneAufgaben > 0}
            <span class="flex items-center gap-2.5">
              <span class="h-1.5 w-1.5 rounded-full bg-orange-300"></span>
              {offeneAufgaben}
              {offeneAufgaben === 1 ? 'Aufgabe' : 'Aufgaben'}
            </span>
          {/if}
          {#if offeneEinkaeufe > 0}
            <span class="flex items-center gap-2.5">
              <span class="h-1.5 w-1.5 rounded-full bg-white/60"></span>
              {offeneEinkaeufe} einzukaufen
            </span>
          {/if}
          {#each heute as e (e.id)}
            <span class="flex items-center gap-2.5">
              <span class="h-1.5 w-1.5 rounded-full bg-primary"></span>
              {e.title}{#if !e.all_day}, {uhr(e.start)}{/if}
            </span>
          {/each}
          {#if offeneAufgaben === 0 && offeneEinkaeufe === 0 && heute.length === 0}
            <span class="text-white/60">Nichts mehr offen</span>
          {/if}
        </div>
      </div>

      <!-- Bedienung erscheint erst bei Berührung -->
      <div
        class="flex items-center gap-2.5 transition-opacity duration-300 {bedienung
          ? 'opacity-100'
          : 'pointer-events-none opacity-0'}"
      >
        <button
          class="flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={() => weiter(-1)}
          aria-label="Vorheriges Bild"
        >
          <ChevronLeft class="h-5 w-5" />
        </button>
        <button
          class="flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={() => (paused = !paused)}
          aria-label={paused ? 'Weiter abspielen' : 'Anhalten'}
        >
          {#if paused}<Play class="h-5 w-5" />{:else}<Pause class="h-5 w-5" />{/if}
        </button>
        <button
          class="flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={() => weiter(1)}
          aria-label="Nächstes Bild"
        >
          <ChevronRight class="h-5 w-5" />
        </button>
        <button
          class="ml-3 flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={beenden}
          aria-label="Diashow beenden"
        >
          <X class="h-5 w-5" />
        </button>
      </div>
    </div>
  </div>

  {#if photos.length > 1}
    <div class="absolute inset-x-0 bottom-0 h-0.5 bg-white/15">
      <div
        class="h-full bg-gradient-to-r from-primary to-amber-300 transition-[width] duration-500"
        style="width: {((index + 1) / photos.length) * 100}%"
      ></div>
    </div>
  {/if}
</div>
