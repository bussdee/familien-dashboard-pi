<script lang="ts">
  import { t } from '$lib/i18n';
  import { onDestroy, onMount } from 'svelte';
  import { BellRing, Plus, Square, Timer as TimerIcon } from 'lucide-svelte';
  import Kachel from './Kachel.svelte';

  /**
   * Ein Küchen-Timer fürs Wandtablet: Nudeln, Zähneputzen, Hausaufgaben,
   * „noch fünf Minuten". Er läuft nur auf diesem Gerät und braucht den Server
   * nicht. Ein Neuladen der Seite stoppt ihn nicht — das Ende steht im
   * Browserspeicher, nicht im Arbeitsspeicher.
   *
   * Klingeln: ein kurzer Ton aus dem Browser, dazu Vibration, wo es sie gibt.
   * Der Ton setzt eine Berührung voraus (Browserregel); die steckt im Antippen
   * von „Start".
   */
  const VORGABEN = [1, 3, 5, 10, 15, 30];
  const SPEICHER = 'timer.ende';

  let endet = $state<number | null>(null);
  let dauer = $state(0);
  let jetzt = $state(Date.now());
  let klingelt = $state(false);
  let eigene = $state('');

  const rest = $derived(endet ? Math.max(0, Math.ceil((endet - jetzt) / 1000)) : 0);
  const anteil = $derived(endet && dauer ? Math.min(100, ((dauer - rest) / dauer) * 100) : 0);
  const laeuft = $derived(endet !== null && !klingelt);

  function mmss(sek: number) {
    const m = Math.floor(sek / 60);
    const s = sek % 60;
    return `${m}:${String(s).padStart(2, '0')}`;
  }

  let ton: AudioContext | null = null;
  let tonTimer: ReturnType<typeof setInterval> | null = null;
  let takt: ReturnType<typeof setInterval> | null = null;

  function piep() {
    try {
      if (!ton) return;
      const o = ton.createOscillator();
      const g = ton.createGain();
      o.type = 'sine';
      o.frequency.value = 880;
      g.gain.setValueAtTime(0.0001, ton.currentTime);
      g.gain.exponentialRampToValueAtTime(0.4, ton.currentTime + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, ton.currentTime + 0.4);
      o.connect(g).connect(ton.destination);
      o.start();
      o.stop(ton.currentTime + 0.45);
      navigator.vibrate?.(200);
    } catch {
      /* Ohne Ton klingelt der Bildschirm eben nur sichtbar. */
    }
  }

  function merken() {
    try {
      if (endet) localStorage.setItem(SPEICHER, JSON.stringify({ endet, dauer }));
      else localStorage.removeItem(SPEICHER);
    } catch {
      /* privater Modus: Der Timer läuft trotzdem, nur ohne Neuladen-Schutz. */
    }
  }

  function starten(minuten: number) {
    if (!(minuten > 0) || minuten > 600) return;
    try {
      ton ??= new AudioContext();
      void ton.resume();
    } catch {
      /* siehe oben */
    }
    dauer = Math.round(minuten * 60);
    endet = Date.now() + dauer * 1000;
    jetzt = Date.now();
    klingelt = false;
    eigene = '';
    merken();
  }

  function dazu(minuten: number) {
    if (!endet) return;
    endet += minuten * 60_000;
    dauer += minuten * 60;
    merken();
  }

  function stoppen() {
    endet = null;
    klingelt = false;
    if (tonTimer) clearInterval(tonTimer);
    tonTimer = null;
    merken();
  }

  function pruefen() {
    jetzt = Date.now();
    if (endet && !klingelt && jetzt >= endet) {
      klingelt = true;
      piep();
      tonTimer = setInterval(piep, 1500);
    }
  }

  onMount(() => {
    try {
      const raw = localStorage.getItem(SPEICHER);
      if (raw) {
        const g = JSON.parse(raw) as { endet: number; dauer: number };
        // Ein Timer, der vor Stunden abgelaufen ist, soll nicht beim
        // Neuladen plötzlich klingeln.
        if (g.endet > Date.now() - 60_000) {
          endet = g.endet;
          dauer = g.dauer;
        } else {
          localStorage.removeItem(SPEICHER);
        }
      }
    } catch {
      /* nichts gemerkt */
    }
    takt = setInterval(pruefen, 250);
  });

  onDestroy(() => {
    if (takt) clearInterval(takt);
    if (tonTimer) clearInterval(tonTimer);
  });
</script>

{#snippet zeile()}
  {#if klingelt}
    {t('Die Zeit ist um!')}
  {:else if laeuft}
    {t('läuft')}
  {:else}
    {t('Wie lange?')}
  {/if}
{/snippet}

<Kachel ton="var(--ton-zeiten)" titel={t('Timer')} icon={TimerIcon} {zeile}>
  {#if endet === null}
    <div class="flex flex-wrap gap-2">
      {#each VORGABEN as m (m)}
        <button class="chip !min-h-[44px] px-4 text-base font-medium" onclick={() => starten(m)}>
          {m} {t('Min.')}
        </button>
      {/each}
    </div>
    <form
      class="mt-3 flex gap-2"
      onsubmit={(e) => {
        e.preventDefault();
        starten(Number.parseFloat(eigene.replace(',', '.')));
      }}
    >
      <input class="input" inputmode="decimal" placeholder={t('Eigene Minuten')} bind:value={eigene} maxlength="5" aria-label={t('Eigene Minuten')} />
      <button class="btn-primary shrink-0" disabled={!(Number.parseFloat(eigene.replace(',', '.')) > 0)}>{t('Start')}</button>
    </form>
  {:else}
    <div class="flex flex-col items-center gap-3 py-2 {klingelt ? 'animate-pulse' : ''}">
      <p class="font-display text-6xl font-light tabular-nums tracking-tight {klingelt ? 'text-destructive' : ''}">
        {mmss(rest)}
      </p>
      <div class="h-2 w-full overflow-hidden rounded-full bg-muted" role="progressbar" aria-valuenow={Math.round(anteil)} aria-valuemin="0" aria-valuemax="100">
        <div class="h-full rounded-full bg-primary transition-[width] duration-300" style="width: {anteil}%"></div>
      </div>
      <div class="flex w-full gap-2">
        {#if klingelt}
          <button class="btn-primary flex-1" onclick={stoppen}><BellRing class="h-4 w-4" /> {t('Okay')}</button>
          <button class="btn-outline px-4" onclick={() => starten(1)}>+1 {t('Min.')}</button>
        {:else}
          <button class="btn-outline flex-1" onclick={() => dazu(1)}><Plus class="h-4 w-4" /> 1 {t('Min.')}</button>
          <button class="btn-outline flex-1" onclick={stoppen}><Square class="h-4 w-4" /> {t('Stopp')}</button>
        {/if}
      </div>
    </div>
  {/if}
</Kachel>
