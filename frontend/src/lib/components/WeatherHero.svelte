<script lang="ts">
  import { format, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import type { WeatherData } from '$lib/types';

  let { weather }: { weather: WeatherData | null } = $props();

  const current = $derived(weather?.current ?? null);
  const hours = $derived((weather?.hourly ?? []).slice(0, 12));

  const uhr = (iso: string) => format(parseISO(iso), 'HH:mm', { locale: de });

  /**
   * Der Satz, auf den es ankommt. Ein Prozentwert beantwortet nicht, ob die
   * Runde noch geht — ein Zeitpunkt tut es.
   */
  const regenSatz = $derived.by(() => {
    const r = weather?.rain;
    if (!r) return null;
    if (r.now) {
      return r.ends_at
        ? { text: `Es regnet — trocken ab ${uhr(r.ends_at)}`, nass: true }
        : { text: 'Es regnet', nass: true };
    }
    if (r.starts_at) return { text: `Regen ab ${uhr(r.starts_at)}`, nass: true };
    if (r.dry_until) return { text: `Trocken bis ${uhr(r.dry_until)}`, nass: false };
    return null;
  });

  // Die Kurve wird aus den Stundenwerten gezeichnet. Feste Maße, damit sie
  // auch dann sauber aussieht, wenn die Werte eng beieinander liegen.
  const BREITE = 520;
  const HOEHE = 62;

  const kurve = $derived.by(() => {
    if (hours.length < 2) return null;
    const temps = hours.map((h) => h.temperature);
    const min = Math.min(...temps);
    const max = Math.max(...temps);
    const spanne = Math.max(max - min, 2); // nie ganz flach zeichnen
    const schritt = BREITE / (hours.length - 1);

    const punkte = hours.map((h, i) => ({
      x: i * schritt,
      y: HOEHE - 10 - ((h.temperature - min) / spanne) * (HOEHE - 26),
      h,
    }));
    const linie = punkte.map((p) => `${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' L ');
    return {
      punkte,
      pfad: `M ${linie}`,
      flaeche: `M ${linie} L ${BREITE} ${HOEHE} L 0 ${HOEHE} Z`,
      // Wo die erste nasse Stunde liegt, kommt eine Markierung hin.
      nass: punkte.find((p) => p.h.precipitation >= 0.1) ?? null,
    };
  });
</script>

{#if current}
  <!--
    Anklickbar wie früher das kleine Wetter-Symbol: Die Wetterseite ist der
    einzige Ort, an dem der Ort eingestellt wird, und in der Kopfleiste steht
    sie bewusst nicht. Ohne diesen Verweis wäre sie gar nicht erreichbar.
  -->
  <a
    href="/wetter"
    class="flex flex-wrap items-end gap-x-8 gap-y-4 rounded-2xl transition-colors hover:bg-muted/20"
    title="Wetterdetails und Ort einstellen"
  >
    <!-- Die Temperatur ist die Zahl, die man aus dem Flur noch lesen soll -->
    <div class="flex items-end gap-4">
      <span class="font-display text-6xl font-light leading-[0.85] tracking-tight sm:text-7xl">
        {Math.round(current.temperature)}°
      </span>
      <div class="pb-1.5">
        {#if regenSatz}
          <p class="text-sm font-normal {regenSatz.nass ? 'text-sky-300' : 'text-primary'}">
            {regenSatz.text}
          </p>
        {/if}
        <p class="mt-0.5 text-xs font-light text-muted-foreground">
          {current.description} · gefühlt {Math.round(current.feels_like)}°
        </p>
        <p class="text-xs font-light text-muted-foreground">
          {weather?.location.name ?? ''}
        </p>
      </div>
    </div>

    {#if kurve}
      <!-- Der Verlauf der nächsten Stunden, als Kurve statt als Tabelle -->
      <div class="min-w-0 flex-1">
        <svg
          viewBox="0 0 {BREITE} {HOEHE}"
          class="h-16 w-full"
          preserveAspectRatio="none"
          role="img"
          aria-label="Temperaturverlauf der nächsten Stunden"
        >
          <defs>
            <linearGradient id="wetterFlaeche" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stop-color="hsl(var(--primary))" stop-opacity="0.28" />
              <stop offset="1" stop-color="hsl(var(--primary))" stop-opacity="0" />
            </linearGradient>
            <linearGradient id="wetterLinie" x1="0" y1="0" x2="1" y2="0">
              <stop offset="0" stop-color="hsl(var(--primary))" />
              <stop offset="0.6" stop-color="#8FC7F0" />
              <stop offset="1" stop-color="#B478DC" />
            </linearGradient>
          </defs>
          <path d={kurve.flaeche} fill="url(#wetterFlaeche)" />
          <path
            d={kurve.pfad}
            fill="none"
            stroke="url(#wetterLinie)"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            vector-effect="non-scaling-stroke"
          />
          {#if kurve.nass}
            <line
              x1={kurve.nass.x}
              y1="2"
              x2={kurve.nass.x}
              y2={HOEHE}
              stroke="#8FC7F0"
              stroke-opacity="0.45"
              stroke-width="1"
              stroke-dasharray="3 4"
              vector-effect="non-scaling-stroke"
            />
          {/if}
        </svg>
        <!--
          Sechs Uhrzeiten in einer Handy-schmalen Zeile ergaben eine
          zusammenhängende Ziffernfolge — „212301030507" statt sechs Zahlen.
          Auf schmalen Bildschirmen bleiben deshalb nur jede zweite stehen,
          und jede bekommt Mindestbreite und Mitte.
        -->
        <div class="flex justify-between text-[11px] font-light tracking-wide text-muted-foreground">
          {#each hours.filter((_, i) => i % 2 === 0) as h, i (h.time)}
            <span
              class="min-w-[2ch] shrink-0 text-center tabular-nums {i % 2 === 1
                ? 'hidden sm:inline'
                : ''}"
            >
              {format(parseISO(h.time), 'HH')}
            </span>
          {/each}
        </div>
      </div>
    {/if}
  </a>
{/if}
