<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { authApi } from '$lib/api';
  import { connection, oberflaeche, session, theme } from '$lib/stores';
  import { layout } from '$lib/stores/layout.svelte';
  import { board } from '$lib/stores/scores.svelte';
  import Header from '$lib/components/Header.svelte';
  import Footer from '$lib/components/Footer.svelte';
  import Celebration from '$lib/components/Celebration.svelte';
  import InstallPrompt from '$lib/components/InstallPrompt.svelte';
  import UpdatePrompt from '$lib/components/UpdatePrompt.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import WerWarDas from '$lib/components/WerWarDas.svelte';

  let { children } = $props();

  const isLogin = $derived($page.url.pathname.startsWith('/login'));

  onMount(() => {
    theme.init();
    oberflaeche.init();

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

  <!-- pb-20 on phones keeps content clear of the bottom navigation bar. -->
  <main class="flex-1 {isLogin ? '' : 'pb-20 md:pb-0'}">
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

  <!-- Outside the isLogin guard: a confirmation can be asked from anywhere. -->
  <ConfirmDialog />
  <WerWarDas />

  {#if !isLogin}
    <Celebration />
    <InstallPrompt />
    <UpdatePrompt />
  {/if}
</div>
