<script lang="ts">
  import { t, tServer } from '$lib/i18n';
  import { format, parseISO } from 'date-fns';
  import type { WeatherData } from '$lib/types';
  import { dauer, fensterSatz, naechstesFenster, zeitpunkt } from '$lib/utils/trockenfenster';

  let { weather }: { weather: WeatherData | null } = $props();

  const current = $derived(weather?.current ?? null);
  const hours = $derived((weather?.hourly ?? []).slice(0, 12));

  /**
   * Der Satz, auf den es ankommt. Ein Prozentwert beantwortet nicht, ob die
   * Runde noch geht — ein Zeitraum tut es. Deshalb steht hier das nächste
   * Trockenfenster und nicht die Regenwahrscheinlichkeit.
   */
  const fenster = $derived(naechstesFenster(weather));
  const trocken = $derived(fensterSatz(fenster));

  /**
   * Die Zeile darunter: wann es nass wird. Das Fenster sagt, wann man kann —
   * das hier sagt, warum es vorher oder nachher nicht geht.
   */
  const regenSatz = $derived.by(() => {
    const r = weather?.rain;
    if (r?.now) return r.ends_at ? t('Es regnet — trocken ab {0}', [zeitpunkt(r.ends_at)]) : t('Es regnet');
    if (r?.starts_at) return t('Regen ab {0}', [zeitpunkt(r.starts_at)]);
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

    // Die nassen Stunden werden hinterlegt — nicht die trockenen. So liest
    // man die Lücken zwischen den Bändern als das, was sie sind: die Zeit,
    // in der man rauskann. Eine einzelne Markierung beim ersten Regen sagte
    // nur, wann es losgeht, nicht wann es wieder aufhört.
    const baender: { x: number; breite: number }[] = [];
    punkte.forEach((p) => {
      if (!p.h.wet) return;
      const von = Math.max(0, p.x - schritt / 2);
      const bis = Math.min(BREITE, p.x + schritt / 2);
      const letztes = baender[baender.length - 1];
      if (letztes && Math.abs(letztes.x + letztes.breite - von) < 0.5) {
        letztes.breite = bis - letztes.x;
      } else {
        baender.push({ x: von, breite: bis - von });
      }
    });

    return {
      punkte,
      pfad: `M ${linie}`,
      flaeche: `M ${linie} L ${BREITE} ${HOEHE} L 0 ${HOEHE} Z`,
      baender,
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
    title={t('Wetterdetails und Ort einstellen')}
  >
    <!-- Die Temperatur ist die Zahl, die man aus dem Flur noch lesen soll -->
    <div class="flex items-end gap-4">
      <span class="font-display text-6xl font-light leading-[0.85] tracking-tight sm:text-7xl">
        {Math.round(current.temperature)}°
      </span>
      <div class="pb-1.5">
        <p class="text-sm font-normal {fenster ? 'text-primary' : 'text-muted-foreground'}">
          {trocken}{#if fenster}<span class="font-light text-muted-foreground"
              >&nbsp;· {dauer(fenster.fenster.hours)}</span
            >{/if}
        </p>
        {#if regenSatz}
          <p class="mt-0.5 text-xs font-light text-sky-300">{regenSatz}</p>
        {/if}
        <p class="mt-0.5 text-xs font-light text-muted-foreground">
          {t('{0} · gefühlt {1}°', [tServer(current.description), Math.round(current.feels_like)])}
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
          aria-label={t('Temperaturverlauf der nächsten Stunden, nasse Stunden hinterlegt')}
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
          {#each kurve.baender as band, i (i)}
            <rect
              x={band.x}
              y="0"
              width={band.breite}
              height={HOEHE}
              fill="#8FC7F0"
              fill-opacity="0.16"
            />
          {/each}
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
