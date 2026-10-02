<script lang="ts">
  import { t, dfLocale, tServer } from '$lib/i18n';
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { format, parseISO } from 'date-fns';
  import {
    ChevronLeft, ChevronRight, Images, Maximize, Minimize, Pause, Play, X,
  } from 'lucide-svelte';
  import { calendarApi, choresApi, photosApi, shoppingApi, weatherApi } from '$lib/api';
  import { diashow } from '$lib/stores/diashow.svelte';
  import { vollbild } from '$lib/vollbild.svelte';
  import { fensterSatz, naechstesFenster, zeitpunkt } from '$lib/utils/trockenfenster';
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
  let hinweisTimer: ReturnType<typeof setTimeout> | null = null;

  /**
   * Der Hinweis „Antippen für Vollbild" gilt der Diashow, die das Wandgerät
   * von selbst öffnet — dort fehlt die Geste, die der Browser für Vollbild
   * verlangt. Er geht weg, sobald jemand den Bildschirm anfasst.
   */
  let vollbildHinweis = $state(false);

  const aktuell = $derived(photos.length > 0 ? photos[index % photos.length] : null);
  const offeneAufgaben = $derived(chores.filter((c) => c.is_due).length);

  const heute = $derived(
    events
      .filter((e) => new Date(e.start).toDateString() === jetzt.toDateString())
      .slice(0, 2),
  );

  /**
   * Im Flur zählt dieselbe Frage wie überall: wann können wir raus. Das
   * Trockenfenster beantwortet sie, der Regen sagt nur, warum gerade nicht.
   */
  const fenster = $derived(naechstesFenster(weather));
  const trockenSatz = $derived(fenster ? fensterSatz(fenster) : null);

  const regenSatz = $derived.by(() => {
    const r = weather?.rain;
    if (r?.now) return r.ends_at ? t('Regen bis {0}', [zeitpunkt(r.ends_at)]) : t('Es regnet');
    if (r?.starts_at) return t('Regen ab {0}', [zeitpunkt(r.starts_at)]);
    return null;
  });

  const uhr = (iso: string) => format(parseISO(iso), 'HH:mm', { locale: dfLocale });

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

  /**
   * Die erste Berührung auf dem Bildschirm ist eine Geste — genau das, was
   * der Browser für Vollbild verlangt. Einmal antippen genügt, dann zieht er
   * seine Adressleiste zurück, auch ohne installierte App.
   */
  async function ersteBeruehrung() {
    if (vollbild.verfuegbar && !vollbild.aktiv) await vollbild.ein();
    vollbildHinweis = false;
    void holeWachschutz();
  }

  function beenden() {
    // Das Vollbild gehört zur Diashow und hört mit ihr auf — sonst stünde
    // das Dashboard danach ohne Leisten da, mit denen man weiterkommt.
    void vollbild.aus();
    goto('/');
  }

  // ---------------------------------------------------------- Wachschutz
  // Ein Wandtablet, das mitten in der Diashow den Bildschirm abschaltet, ist
  // kein Bilderrahmen. Der Wake Lock hält das Display wach, solange die Show
  // läuft — und gibt es wieder frei, sobald die Seite verlassen wird.

  interface Wachschutz {
    release: () => Promise<void>;
    addEventListener: (typ: 'release', rueckruf: () => void) => void;
  }

  let wachschutz: Wachschutz | null = null;

  async function holeWachschutz() {
    if (!('wakeLock' in navigator) || wachschutz) return;
    try {
      // Der Typ steckt hinter einer Fähigkeitsprüfung; je nach lib-Version
      // kennt TypeScript ihn noch nicht.
      wachschutz = await (
        navigator as Navigator & {
          wakeLock: { request: (typ: 'screen') => Promise<Wachschutz> };
        }
      ).wakeLock.request('screen');
      wachschutz.addEventListener('release', () => (wachschutz = null));
    } catch {
      // Nicht schlimm — dann geht der Bildschirm eben irgendwann aus.
    }
  }

  function beiSichtbarkeit() {
    // Nach einem Blick aufs Handy oder in eine andere App gibt der Browser
    // den Lock frei. Beim Zurückkommen neu anfordern.
    if (document.visibilityState === 'visible') void holeWachschutz();
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
    void holeWachschutz();
    document.addEventListener('visibilitychange', beiSichtbarkeit);

    // Läuft das Vollbild schon (Start über den Diashow-Knopf), braucht es
    // keinen Hinweis. Sonst leise zeigen, wo die Geste hin soll.
    if (vollbild.verfuegbar && !vollbild.aktiv) {
      vollbildHinweis = true;
      hinweisTimer = setTimeout(() => (vollbildHinweis = false), 8000);
    }
  });

  onDestroy(() => {
    if (bildTimer) clearInterval(bildTimer);
    if (uhrTimer) clearInterval(uhrTimer);
    if (datenTimer) clearInterval(datenTimer);
    if (bedienTimer) clearTimeout(bedienTimer);
    if (hinweisTimer) clearTimeout(hinweisTimer);
    document.removeEventListener('visibilitychange', beiSichtbarkeit);
    void wachschutz?.release();
  });

  $effect(() => {
    paused;
    starteBildwechsel();
  });
</script>

<svelte:head>
  <title>{t('Diashow · Familien Dashboard')}</title>
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
  onpointerdown={() => {
    zeigeBedienung();
    void ersteBeruehrung();
  }}
  role="presentation"
>
  {#if aktuell}
    {#key aktuell.name}
      <!--
        Ein weichgezeichneter, formatfüllender Hintergrund — damit die Ränder
        neben einem Hochkantbild nicht als schwarze Balken dastehen.
      -->
      <img
        src={photosApi.url(aktuell.name)}
        alt=""
        aria-hidden="true"
        class="absolute inset-0 h-full w-full scale-110 object-cover opacity-30 blur-2xl"
      />
      <!--
        object-contain statt object-cover: Bei einem Hochkantfoto auf einem
        querformatigen Bildschirm schnitt cover alles bis auf einen Streifen
        aus der Bildmitte weg. Man sah Bauchnabel und Kinn, aber kein Gesicht.
      -->
      <img
        src={photosApi.url(aktuell.name)}
        alt=""
        class="absolute inset-0 h-full w-full animate-fade-in object-contain"
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
      <p class="text-sm">{t('Noch keine Fotos im Rahmen')}</p>
      <a href="/" class="mt-2 text-sm text-primary hover:underline">{t('Zurück zur Übersicht')}</a>
    </div>
  {/if}

  <!-- Uhrzeit gehört auf ein Wandtablet, auch wenn niemand hinsieht -->
  <div class="absolute left-8 top-7 sm:left-11 sm:top-9">
    <div class="font-display text-6xl font-light leading-none tracking-tight sm:text-7xl">
      {format(jetzt, 'HH:mm')}
    </div>
    <div class="mt-2 text-sm font-light text-white/70">
      {format(jetzt, t('EEEE, d. MMMM'), { locale: dfLocale })}
    </div>
  </div>

  {#if weather}
    <div class="absolute right-8 top-7 text-right sm:right-11 sm:top-9">
      <div class="font-display text-4xl font-light leading-none sm:text-5xl">
        {Math.round(weather.current.temperature)}°
      </div>
      {#if trockenSatz}
        <div class="mt-1.5 text-sm font-light text-emerald-200">{trockenSatz}</div>
      {/if}
      {#if regenSatz}
        <div class="mt-0.5 text-sm font-light text-sky-200">{regenSatz}</div>
      {:else if !trockenSatz}
        <div class="mt-1.5 text-sm font-light text-white/65">{tServer(weather.current.description)}</div>
      {/if}
    </div>
  {/if}

  <!-- Was heute noch ansteht: der Grund, warum das Tablet dort hängt -->
  <div class="absolute bottom-8 left-8 right-8 sm:bottom-10 sm:left-11 sm:right-11">
    <div class="flex flex-wrap items-end justify-between gap-6">
      <div class="min-w-0">
        <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-white/50">{t('Heute noch')}</p>
        <div class="mt-3 flex flex-wrap items-center gap-x-7 gap-y-2 text-base font-light sm:text-lg">
          {#if offeneAufgaben > 0}
            <span class="flex items-center gap-2.5">
              <span class="h-1.5 w-1.5 rounded-full bg-orange-300"></span>
              {offeneAufgaben}
              {offeneAufgaben === 1 ? t('Aufgabe') : t('Aufgaben')}
            </span>
          {/if}
          {#if offeneEinkaeufe > 0}
            <span class="flex items-center gap-2.5">
              <span class="h-1.5 w-1.5 rounded-full bg-white/60"></span>
              {t('{0} einzukaufen', [offeneEinkaeufe])}
            </span>
          {/if}
          {#each heute as e (e.id)}
            <span class="flex items-center gap-2.5">
              <span class="h-1.5 w-1.5 rounded-full bg-primary"></span>
              {e.title}{#if !e.all_day}, {uhr(e.start)}{/if}
            </span>
          {/each}
          {#if offeneAufgaben === 0 && offeneEinkaeufe === 0 && heute.length === 0}
            <span class="text-white/60">{t('Nichts mehr offen')}</span>
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
          aria-label={t('Vorheriges Bild')}
        >
          <ChevronLeft class="h-5 w-5" />
        </button>
        <button
          class="flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={() => (paused = !paused)}
          aria-label={paused ? t('Weiter abspielen') : t('Anhalten')}
        >
          {#if paused}<Play class="h-5 w-5" />{:else}<Pause class="h-5 w-5" />{/if}
        </button>
        <button
          class="flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={() => weiter(1)}
          aria-label={t('Nächstes Bild')}
        >
          <ChevronRight class="h-5 w-5" />
        </button>
        {#if vollbild.verfuegbar}
          <button
            class="flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
            onclick={() => vollbild.umschalten()}
            aria-label={vollbild.aktiv ? t('Vollbild verlassen') : t('Vollbild starten')}
            title={vollbild.aktiv ? t('Vollbild verlassen') : t('Vollbild starten')}
          >
            {#if vollbild.aktiv}<Minimize class="h-5 w-5" />{:else}<Maximize class="h-5 w-5" />{/if}
          </button>
        {/if}
        <button
          class="ml-3 flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur transition-colors hover:bg-white/25"
          onclick={beenden}
          aria-label={t('Diashow beenden')}
        >
          <X class="h-5 w-5" />
        </button>
      </div>
    </div>
  </div>

  <!--
    Hinweis für die Diashow, die das Wandgerät selbst öffnet: Vollbild
    braucht eine Berührung, und dieser Zeiger sagt leise, wo sie hin soll.
  -->
  {#if vollbildHinweis}
    <div class="pointer-events-none absolute inset-x-0 bottom-24 flex justify-center">
      <p class="rounded-full bg-black/50 px-4 py-1.5 text-xs text-white/70 backdrop-blur">
        {t('Antippen für Vollbild')}
      </p>
    </div>
  {/if}

  <!--
    Geräte ohne Vollbild-API (iPhone Safari) bekommen keinen toten Knopf,
    sondern den Weg, der dort funktioniert: als App auf dem Home-Bildschirm
    läuft die Seite ohne Adressleiste.
  -->
  {#if !vollbild.verfuegbar && bedienung}
    <div class="pointer-events-none absolute inset-x-0 bottom-24 flex justify-center">
      <p class="rounded-full bg-black/50 px-4 py-1.5 text-xs text-white/70 backdrop-blur">
        {t('Ohne Adressleiste: „Zum Home-Bildschirm“ hinzufügen')}
      </p>
    </div>
  {/if}

  {#if photos.length > 1}
    <div class="absolute inset-x-0 bottom-0 h-0.5 bg-white/15">
      <div
        class="h-full bg-gradient-to-r from-primary to-amber-300 transition-[width] duration-500"
        style="width: {((index + 1) / photos.length) * 100}%"
      ></div>
    </div>
  {/if}
</div>
