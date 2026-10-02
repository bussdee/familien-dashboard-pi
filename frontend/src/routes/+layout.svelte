<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { authApi } from '$lib/api';
  import { akzent, connection, oberflaeche, session, theme } from '$lib/stores';
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
  import BottomNav from '$lib/components/BottomNav.svelte';
  import QuickAdd from '$lib/components/QuickAdd.svelte';
  import Toasts from '$lib/components/Toasts.svelte';
  import PunkteBuchen from '$lib/components/PunkteBuchen.svelte';
  import ElternPin from '$lib/components/ElternPin.svelte';

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

  /**
   * Die Leiste am unteren Rand und das Plus gibt es überall ausser beim
   * Anmelden und in der Diashow — dort gehört das ganze Bild dem Foto.
   */
  const mitNavigation = $derived(!isLogin && !inDiashow && $session.ready);

  /**
   * Das Seitenende muss über allem liegen, was unten schwebt: der Leiste
   * (auf dem Handy) und der Abspielleiste. Die Höhe der Leiste steht als
   * --unten-leiste in app.css — hier wird nur addiert, statt für jede
   * Kombination eine eigene Klasse zu raten.
   */
  const abstandUnten = $derived(
    isLogin
      ? ''
      : `padding-bottom: calc(var(--unten-leiste, 0px) + ${player.aktiv ? '5.5rem' : '1rem'})`,
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
    akzent.init();
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

<!-- Der Abstand unten sitzt am äussersten Rahmen und nicht an <main>:
     Sonst läge die Fusszeile hinter der Leiste. -->
<div
  class="flex min-h-full flex-col bg-background {mitNavigation ? 'mit-unten-leiste' : ''}"
  style={abstandUnten}
>
  {#if !isLogin}
    <Header />
  {/if}

  <main class="flex-1">
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

  {#if mitNavigation}
    <BottomNav />
    <QuickAdd />
  {/if}

  <Toasts />

  <!-- Punkte vergeben ist Elternsache. Am Wandgerät geht es mit der PIN
       eines Elternteils, die ElternPin dort abfragt. -->
  {#if $session.user?.role === 'admin' || $session.device}
    <PunkteBuchen />
  {/if}
  {#if $session.device}
    <ElternPin />
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
