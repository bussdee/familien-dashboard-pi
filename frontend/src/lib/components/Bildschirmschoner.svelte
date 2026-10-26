<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { session } from '$lib/stores';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { diashow } from '$lib/stores/diashow.svelte';

  /**
   * Der Bildschirmschoner: Nach einer Weile ohne Berührung öffnet sich die
   * Diashow (oder, ohne Fotos, eine grosse Uhr). Eine Berührung bringt die
   * Seite zurück, auf der man war.
   *
   * Wann, für wen und wie lange stellen die Eltern in der Verwaltung ein
   * (Bereich „Bildschirm"). Vorgabe: das Wandgerät nach fünf Minuten — so
   * verhielt es sich schon vorher, nur auf der Übersicht und ohne Einstellung.
   *
   * Als „Nutzung" zählt jede Berührung, Taste und jedes Rad; ein Eingabefeld
   * mit dem Fokus zählt ebenfalls: Wer gerade tippt, soll nicht mitten im Satz
   * von einer Diashow unterbrochen werden.
   */
  let zuletzt = Date.now();

  function beruehrt() {
    zuletzt = Date.now();
  }

  const aufDiashow = $derived($page.url.pathname.startsWith('/diashow'));
  const aufLogin = $derived($page.url.pathname.startsWith('/login'));

  const gilt = $derived.by(() => {
    const modus = einstellungen.werte?.['schoner.modus'] ?? 'wandgeraet';
    if (!$session.ready || aufDiashow || aufLogin) return false;
    if (modus === 'alle') return true;
    if (modus === 'wandgeraet') return $session.device;
    return false;
  });

  const wartezeitMs = $derived(
    Math.max(1, Number.parseInt(einstellungen.werte?.['schoner.leerlauf_min'] ?? '5', 10) || 5) * 60_000,
  );

  function pruefen() {
    if (!gilt || document.visibilityState !== 'visible') return;
    const el = document.activeElement;
    if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) {
      zuletzt = Date.now();
      return;
    }
    if (Date.now() - zuletzt < wartezeitMs) return;
    diashow.starten($page.url.pathname + $page.url.search);
    zuletzt = Date.now();
    void goto('/diashow');
  }

  onMount(() => {
    const takt = setInterval(pruefen, 10_000);
    return () => clearInterval(takt);
  });
</script>

<svelte:window
  onpointerdown={beruehrt}
  onpointermove={beruehrt}
  onkeydown={beruehrt}
  onwheel={beruehrt}
  ontouchstart={beruehrt}
/>
