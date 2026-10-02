<script lang="ts">
  import { t, dfLocale, tServer } from '$lib/i18n';
  import { onMount } from 'svelte';
  import {
    ArrowLeft, Check, Cloud, CloudDrizzle, CloudFog, CloudLightning, CloudRain,
    CloudSnow, CloudSun, Droplets, LoaderCircle, MapPin, Search, Sun, Sunrise,
    Sunset, Thermometer, Wind, X,
  } from 'lucide-svelte';
  import { format, parseISO } from 'date-fns';
  import { ApiError, weatherApi } from '$lib/api';
  import { session } from '$lib/stores';
  import type { WeatherData, WeatherLocation, WeatherWindow } from '$lib/types';
  import { alleFenster, dauer, fensterBeschreibung, uhr } from '$lib/utils/trockenfenster';

  let weather = $state<WeatherData | null>(null);
  let loading = $state(true);
  let loadError = $state('');

  const isAdmin = $derived($session.user?.role === 'admin');

  const icons: Record<string, typeof Cloud> = {
    sun: Sun,
    'cloud-sun': CloudSun,
    cloud: Cloud,
    'cloud-drizzle': CloudDrizzle,
    'cloud-rain': CloudRain,
    'cloud-snow': CloudSnow,
    'cloud-fog': CloudFog,
    'cloud-lightning': CloudLightning,
  };
  const iconFor = (name: string) => icons[name] ?? Cloud;

  const dayLabel = (iso: string, index: number) =>
    index === 0 ? t('Heute') : index === 1 ? t('Morgen') : format(parseISO(iso), 'EEEE', { locale: dfLocale });

  const clock = (iso: string) => {
    try {
      return format(parseISO(iso), 'HH:mm');
    } catch {
      return '—';
    }
  };

  /**
   * Die Trockenfenster der ganzen Vorhersage, aber nicht alle auf einmal:
   * Wer bis Freitag plant, plant nicht nach dieser Seite. Sechs sind genug,
   * um heute und morgen vollständig zu sehen.
   */
  const fenster = $derived(alleFenster(weather).slice(0, 6));

  /** Die Stunden, ab jetzt. Die Liste kommt schon ab der aktuellen Stunde. */
  const stunden = $derived(weather?.hourly ?? []);

  /** Wie stark eine Stunde hinterlegt wird — je nasser, desto deutlicher. */
  function nassStufe(prob: number): string {
    if (prob >= 80) return 'bg-sky-500/25';
    if (prob >= 55) return 'bg-sky-500/15';
    return 'bg-sky-500/[0.07]';
  }

  function locationLabel(loc: WeatherLocation | undefined): string {
    if (!loc) return '';
    return [loc.name, loc.region !== loc.name ? loc.region : '', loc.country]
      .filter(Boolean)
      .join(', ');
  }

  async function load() {
    try {
      weather = await weatherApi.get();
      loadError = '';
    } catch (e) {
      loadError = e instanceof ApiError ? e.message : t('Wetter konnte nicht geladen werden');
    } finally {
      loading = false;
    }
  }

  // ---- Ortssuche ----
  let picking = $state(false);
  let query = $state('');
  let results = $state<WeatherLocation[]>([]);
  let searching = $state(false);
  let saving = $state(false);
  let searchError = $state('');
  let searchTimer: ReturnType<typeof setTimeout> | null = null;

  // Debounced so typing "Salzburg" is one request, not eight.
  function onQueryInput() {
    if (searchTimer) clearTimeout(searchTimer);
    searchError = '';
    if (query.trim().length < 2) {
      results = [];
      return;
    }
    searchTimer = setTimeout(runSearch, 350);
  }

  async function runSearch() {
    searching = true;
    try {
      results = await weatherApi.search(query.trim());
      if (results.length === 0) searchError = t('Kein Ort gefunden für „{0}"', [query.trim()]);
    } catch (e) {
      searchError = e instanceof ApiError ? e.message : t('Ortssuche fehlgeschlagen');
      results = [];
    } finally {
      searching = false;
    }
  }

  async function choose(loc: WeatherLocation) {
    saving = true;
    searchError = '';
    try {
      await weatherApi.setLocation(loc);
      picking = false;
      query = '';
      results = [];
      // The backend refetches straight away; give it a moment to land.
      await new Promise((r) => setTimeout(r, 900));
      await load();
    } catch (e) {
      searchError = e instanceof ApiError ? e.message : t('Standort konnte nicht gespeichert werden');
    } finally {
      saving = false;
    }
  }

  onMount(load);
</script>

<svelte:head><title>{t('Wetter · Familien Dashboard')}</title></svelte:head>

<div class="mx-auto max-w-2xl px-4 py-5">
  <a
    href="/"
    class="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
  >
    <ArrowLeft class="h-4 w-4" /> {t('Übersicht')}
  </a>

  {#if loading}
    <div class="card h-64 animate-pulse bg-muted/40"></div>
  {:else if !weather}
    <section class="card p-8 text-center">
      <Cloud class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-40" />
      <p class="font-medium">{t('Keine Wetterdaten')}</p>
      <p class="mt-1 text-sm text-muted-foreground">{loadError}</p>
      <button class="btn-outline mt-4" onclick={load}>{t('Erneut versuchen')}</button>
    </section>
  {:else}
    {@const Icon = iconFor(weather.current.icon)}

    <section class="card mb-4 p-6">
      <button
        class="mb-4 flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground disabled:cursor-default disabled:hover:text-muted-foreground"
        onclick={() => isAdmin && (picking = !picking)}
        disabled={!isAdmin}
        title={isAdmin ? t('Ort ändern') : t('Ort ändert ein Administrator')}
      >
        <MapPin class="h-4 w-4 shrink-0" />
        {locationLabel(weather.location) || t('Ort wählen')}
      </button>

      {#if picking}
        <div class="mb-5 rounded-xl border border-border p-3">
          <div class="mb-2 flex items-center justify-between">
            <p class="text-sm font-medium">{t('Ort suchen')}</p>
            <button
              class="touch-target text-muted-foreground"
              onclick={() => (picking = false)}
              aria-label={t('Schließen')}
            >
              <X class="h-4 w-4" />
            </button>
          </div>

          <div class="relative">
            <Search
              class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            />
            <input
              class="input pl-9"
              placeholder={t('z. B. Wien, Salzburg…')}
              bind:value={query}
              oninput={onQueryInput}
            />
            {#if searching}
              <LoaderCircle
                class="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-muted-foreground"
              />
            {/if}
          </div>

          {#if searchError}
            <p class="mt-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {searchError}
            </p>
          {/if}

          {#if results.length > 0}
            <ul class="scrollbar-thin mt-2 max-h-56 space-y-1 overflow-y-auto">
              {#each results as loc (`${loc.latitude},${loc.longitude}`)}
                <li>
                  <button
                    class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors hover:bg-accent disabled:opacity-50"
                    onclick={() => choose(loc)}
                    disabled={saving}
                  >
                    <MapPin class="h-4 w-4 shrink-0 text-muted-foreground" />
                    <span class="min-w-0 flex-1">
                      <span class="block truncate font-medium">{loc.name}</span>
                      <span class="block truncate text-xs text-muted-foreground">
                        {[loc.region, loc.country].filter(Boolean).join(', ')}
                      </span>
                    </span>
                    {#if weather.location.latitude === loc.latitude && weather.location.longitude === loc.longitude}
                      <Check class="h-4 w-4 shrink-0 text-primary" />
                    {/if}
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}

      <div class="flex items-center gap-5">
        <Icon class="h-20 w-20 shrink-0 text-primary" />
        <div class="min-w-0">
          <p class="text-5xl font-bold tabular-nums">
            {Math.round(weather.current.temperature)}°
          </p>
          <p class="text-lg">{tServer(weather.current.description)}</p>
          <p class="text-sm text-muted-foreground">
            {t('gefühlt {0}°', [Math.round(weather.current.feels_like)])}
          </p>
        </div>
      </div>

      <div class="mt-5 grid grid-cols-3 gap-2 border-t border-border pt-4 text-center">
        <div>
          <Wind class="mx-auto mb-1 h-4 w-4 text-muted-foreground" />
          <p class="text-sm font-medium tabular-nums">
            {weather.current.wind_speed.toFixed(0)} km/h
          </p>
          <p class="text-[11px] text-muted-foreground">{t('Wind')}</p>
        </div>
        <div>
          <Droplets class="mx-auto mb-1 h-4 w-4 text-muted-foreground" />
          <p class="text-sm font-medium tabular-nums">{weather.current.humidity}%</p>
          <p class="text-[11px] text-muted-foreground">{t('Luftfeuchte')}</p>
        </div>
        <div>
          <Thermometer class="mx-auto mb-1 h-4 w-4 text-muted-foreground" />
          <p class="text-sm font-medium tabular-nums">
            {Math.round(weather.forecast[0]?.temp_max ?? 0)}° / {Math.round(
              weather.forecast[0]?.temp_min ?? 0,
            )}°
          </p>
          <p class="text-[11px] text-muted-foreground">{t('Heute')}</p>
        </div>
      </div>

      {#if weather.stale}
        <p class="mt-4 rounded-lg bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
          {t('Offline – zeigt die zuletzt gespeicherten Daten von {0} Uhr.', [format(parseISO(weather.updated), t('dd.MM. HH:mm'))])}
        </p>
      {:else}
        <p class="mt-4 text-xs text-muted-foreground">
          {t('Aktualisiert um {0} Uhr', [format(parseISO(weather.updated), 'HH:mm')])}
        </p>
      {/if}
    </section>

    <!--
      Die Frage, mit der diese Seite aufgerufen wird, lautet nicht „wie warm
      wird es" — sie lautet „wann können wir raus". Also steht die Antwort
      darauf oben, noch vor der Tagesliste.
    -->
    <section class="card mb-4">
      <div class="p-4 pb-3">
        <h2 class="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          {t('Trockenfenster')}
        </h2>
        <p class="mt-1 text-xs text-muted-foreground">
          {t('Zeiträume ohne Regen, zwischen Sonnenaufgang und Sonnenuntergang.')}
        </p>
      </div>

      {#if fenster.length === 0}
        <p class="border-t border-border p-4 text-sm text-muted-foreground">
          {t('In der Vorhersage ist kein trockener Zeitraum von mindestens anderthalb Stunden dabei.')}
        </p>
      {:else}
        <ul class="divide-y divide-border border-t border-border">
          {#each fenster as f (f.tag.date + f.fenster.from)}
            {@const FensterIcon = f.fenster.sunny ? Sun : Cloud}
            <li class="flex items-center gap-3 p-4">
              <FensterIcon
                class="h-6 w-6 shrink-0 {f.fenster.sunny
                  ? 'text-amber-500'
                  : 'text-muted-foreground'}"
              />
              <div class="min-w-0 flex-1">
                <p class="flex flex-wrap items-center gap-x-2 text-sm font-medium">
                  <span class="capitalize">{dayLabel(f.tag.date, f.tagIndex)}</span>
                  <span class="tabular-nums">{t('{0}–{1} Uhr', [uhr(f.fenster.from), uhr(f.fenster.to)])}</span>
                  {#if f.fenster.now}
                    <span
                      class="rounded-full bg-primary/15 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-primary"
                    >
                      {t('läuft')}
                    </span>
                  {/if}
                </p>
                <p class="mt-0.5 text-[11px] text-muted-foreground">
                  {fensterBeschreibung(f.fenster)}
                </p>
              </div>
              <span class="shrink-0 text-sm font-semibold tabular-nums">
                {dauer(f.fenster.hours)}
              </span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    {#if stunden.length > 0}
      <!--
        Stunde für Stunde, damit man das Fenster oben nachvollziehen kann.
        Die nassen Stunden sind hinterlegt — je kräftiger, desto sicherer der
        Regen. Die hellen Lücken dazwischen sind die Fenster.
      -->
      <section class="card mb-4">
        <h2 class="p-4 pb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          {t('Stunde für Stunde')}
        </h2>
        <div class="scrollbar-thin flex gap-1 overflow-x-auto px-4 pb-4">
          {#each stunden as h (h.time)}
            {@const StundenIcon = iconFor(h.icon)}
            <div
              class="flex w-14 shrink-0 flex-col items-center gap-1 rounded-lg py-2 {h.wet
                ? nassStufe(h.precip_probability)
                : ''}"
            >
              <span class="text-[11px] tabular-nums text-muted-foreground">
                {format(parseISO(h.time), 'HH')}
              </span>
              <StundenIcon class="h-4 w-4 {h.wet ? 'text-sky-500' : 'text-primary'}" />
              <span class="text-sm font-medium tabular-nums">{Math.round(h.temperature)}°</span>
              <span class="h-3.5 text-[10px] tabular-nums text-sky-600 dark:text-sky-400">
                {#if h.precip_probability >= 10}{h.precip_probability}%{/if}
              </span>
            </div>
          {/each}
        </div>
        <p class="px-4 pb-4 text-[11px] text-muted-foreground">
          {t('Hinterlegt: Stunden, in denen Regen zu erwarten ist.')}
        </p>
      </section>
    {/if}

    <section class="card divide-y divide-border">
      <h2 class="p-4 pb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
        {t('Die nächsten Tage')}
      </h2>
      {#each weather.forecast as day, i (day.date)}
        {@const DayIcon = iconFor(day.icon)}
        <div class="flex items-center gap-3 p-4">
          <span class="w-24 shrink-0 text-sm font-medium capitalize">{dayLabel(day.date, i)}</span>
          <DayIcon class="h-7 w-7 shrink-0 text-primary" />

          <div class="min-w-0 flex-1">
            <p class="truncate text-sm">{tServer(day.description)}</p>
            {#if (day.windows ?? []).length > 0}
              <p class="truncate text-[11px] text-primary">
                {t('Trocken {0}', [(day.windows ?? [])
                  .map((w: WeatherWindow) => `${uhr(w.from)}–${uhr(w.to)}`)
                  .join(' · ')])}
              </p>
            {/if}
            <p class="flex flex-wrap items-center gap-x-3 text-[11px] text-muted-foreground">
              {#if day.precip_probability > 0}
                <span class="text-sky-600 dark:text-sky-400">💧 {day.precip_probability}%</span>
              {/if}
              {#if day.sunrise}
                <span class="inline-flex items-center gap-1">
                  <Sunrise class="h-3 w-3" />{clock(day.sunrise)}
                </span>
              {/if}
              {#if day.sunset}
                <span class="inline-flex items-center gap-1">
                  <Sunset class="h-3 w-3" />{clock(day.sunset)}
                </span>
              {/if}
            </p>
          </div>

          <div class="shrink-0 text-right">
            <span class="text-base font-semibold tabular-nums">{Math.round(day.temp_max)}°</span>
            <span class="ml-1 text-sm tabular-nums text-muted-foreground">
              {Math.round(day.temp_min)}°
            </span>
          </div>
        </div>
      {/each}
    </section>

    <p class="mt-4 text-center text-xs text-muted-foreground">
      {t('Daten von Open-Meteo · kein Konto, kein API-Schlüssel')}
    </p>
  {/if}
</div>
