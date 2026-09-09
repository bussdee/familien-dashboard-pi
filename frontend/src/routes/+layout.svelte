<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { authApi } from '$lib/api';
  import { connection, oberflaeche, session, theme } from '$lib/stores';
  import { layout } from '$lib/stores/layout.svelte';
  import { player } from '$lib/stores/player.svelte';
  import { diashow } from '$lib/stores/diashow.svelte';
  import { board } from '$lib/stores/scores.svelte';
  import Header from '$lib/components/Header.svelte';
  import Footer from '$lib/components/Footer.svelte';
  import Celebration from '$lib/components/Celebration.svelte';
  import InstallPrompt from '$lib/components/InstallPrompt.svelte';
  import UpdatePrompt from '$lib/components/UpdatePrompt.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import WerWarDas from '$lib/components/WerWarDas.svelte';
  import PlayerBar from '$lib/components/PlayerBar.svelte';

  let { children } = $props();

  const isLogin = $derived($page.url.pathname.startsWith('/login'));

  /**
   * Das eine <audio>-Element der ganzen App. Es steht hier und nicht in der
   * Musik-Kachel: Sonst bräche die Musik ab, sobald jemand auf „Rangliste"
   * tippt — SvelteKit baut die Seite darunter neu auf, das Seitenlayout aber
   * nicht.
   */
  let audioEl: HTMLAudioElement | null = $state(null);

  /**
   * Die Diashow bedient sich selbst: Dort verschwinden alle Knöpfe, solange
   * niemand das Bild berührt. Die Leiste macht das mit, statt als heller
   * Streifen unter dem Vollbild stehenzubleiben.
   */
  const inDiashow = $derived($page.url.pathname.startsWith('/diashow'));

  const abstandUnten = $derived(
    isLogin ? '' : player.aktiv ? 'pb-36 md:pb-24' : 'pb-20 md:pb-0',
  );

  // Die Diashow liegt als Vollbild über allem. Der Zustand wird beim Betreten
  // und Verlassen hier gesetzt, damit die Seite selbst nichts davon wissen
  // muss, wann sie verlassen wird.
  $effect(() => {
    if (inDiashow) diashow.betreten();
    else diashow.verlassen();
  });

  onMount(() => {
    theme.init();
    oberflaeche.init();
    player.init();
    if (audioEl) player.attach(audioEl);

    // Resolve the session once, then let each page render. Without this gate
    // the dashboard would flash before we know whether anyone is signed in.
    authApi
      .me()
      .then((antwort) => {
        // Am Wandgerät antwortet der Server mit {device:true} statt mit einer
        // Person. Das ist kein Fehler, sondern der Familien-Modus.
        if ('device' in antwort) {
          session.setDevice();
          void layout.load();
          void board.refresh().catch(() => {});
          return;
        }
        session.set(antwort);
        void layout.load();
        // The header shows the signed-in person's score on every page, so the
        // board is loaded once here rather than only on the dashboard.
        void board.refresh().catch(() => {});
      })
      .catch(() => {
        session.clear();
        if (!location.pathname.startsWith('/login')) void goto('/login');
      });

    // Registered explicitly rather than relying on SvelteKit's automatic hook,
    // which did not emit for this adapter/build combination. register() on the
    // same URL is idempotent, so an added auto-registration would be harmless.
    if ('serviceWorker' in navigator && window.isSecureContext) {
      navigator.serviceWorker.register('/service-worker.js', { scope: '/' }).catch(() => {
        /* not fatal — the dashboard works without the offline cache */
      });
    }

    const on = () => connection.setOnline(true);
    const off = () => connection.setOnline(false);
    window.addEventListener('online', on);
    window.addEventListener('offline', off);


    return () => {
      window.removeEventListener('online', on);
      window.removeEventListener('offline', off);
    };
  });
</script>

<div class="flex min-h-full flex-col bg-background">
  {#if !isLogin}
    <Header />
  {/if}

  <!--
    pb-20 on phones keeps content clear of the bottom navigation bar. Läuft
    Musik, kommt die Höhe der Abspielleiste dazu — sonst liegt sie über der
    letzten Kachel.

    Genau EINE Abstandsklasse, nicht zwei sich widersprechende: Welche von
    "pb-20" und "pb-36" gewinnt, entscheidet sonst die Reihenfolge im
    erzeugten Stylesheet und nicht diese Datei.
  -->
  <main class="flex-1 {abstandUnten}">
    {#if $session.ready || isLogin}
      {@render children()}
    {:else}
      <div class="flex items-center justify-center py-24">
        <div class="flex flex-col items-center gap-3 text-muted-foreground">
          <div
            class="w-8 h-8 rounded-full border-2 border-border border-t-primary animate-spin"
          ></div>
          <p class="text-sm">Lade Dashboard…</p>
        </div>
      </div>
    {/if}
  </main>

  {#if !isLogin}
    <Footer />
  {/if}

  <!--
    Ein einziges Audio-Element für die ganze App. preload="none" ist Absicht:
    Bei einer Sammlung mit Hörspielen soll der Browser nicht von sich aus
    Megabytes ziehen, bevor jemand auf Abspielen tippt.
  -->
  <audio bind:this={audioEl} preload="none" class="hidden"></audio>

  {#if !isLogin}
    <PlayerBar gedimmt={inDiashow && !diashow.wach} ueberDiashow={inDiashow} />
  {/if}

  <!-- Outside the isLogin guard: a confirmation can be asked from anywhere. -->
  <ConfirmDialog />
  <WerWarDas />

  {#if !isLogin}
    <Celebration />
    <InstallPrompt />
    <UpdatePrompt />
  {/if}
</div>
