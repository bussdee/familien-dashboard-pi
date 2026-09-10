<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { authApi } from '$lib/api';
  import { connection, session, theme, type Theme } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import {
    Clock, CloudSun, House, LayoutGrid, Link as LinkIcon, LogOut, Menu, Monitor,
    Moon, Settings, Shield, Sun, Trophy, WifiOff, X, UserRound,
  } from 'lucide-svelte';

  let menuOpen = $state(false);

  const themes: { value: Theme; label: string; icon: typeof Sun }[] = [
    { value: 'light', label: 'Hell', icon: Sun },
    { value: 'dark', label: 'Dunkel', icon: Moon },
    { value: 'system', label: 'System', icon: Monitor },
  ];

  const user = $derived($session.user);
  const path = $derived($page.url.pathname);
  const me = $derived(board.for(user?.id));

  const geraet = $derived($session.device);

  // Im Familien-Modus verschwindet alles, was einer Person gehört: Links,
  // eigene Ansicht, Einstellungen — und die Rangliste, die im Flur niemanden
  // etwas angeht.
  const nav = $derived(
    [
      { href: '/', label: 'Übersicht', icon: House, show: true },
      { href: '/wetter', label: 'Wetter', icon: CloudSun, show: true },
      { href: '/links', label: 'Links', icon: LinkIcon, show: !geraet },
      { href: '/zeiten', label: 'Arbeit & Schule', icon: Clock, show: !geraet },
      { href: '/rangliste', label: 'Rangliste', icon: Trophy, show: !geraet },
      { href: '/ansicht', label: 'Ansicht anpassen', icon: LayoutGrid, show: !geraet },
      { href: '/settings', label: 'Einstellungen', icon: Settings, show: !geraet },
      { href: '/admin', label: 'Verwaltung', icon: Shield, show: user?.role === 'admin' },
    ].filter((item) => item.show),
  );

  // Only the top three ranks get a medal; below that the number carries it.
  const medals = ['🥇', '🥈', '🥉'];
  const myMedal = $derived(
    me && me.total_points > 0 ? (medals[me.rank - 1] ?? `${me.rank}.`) : '',
  );

  // Any navigation closes the drawer, including the browser's back button.
  $effect(() => {
    void path;
    menuOpen = false;
  });

  async function signOut() {
    menuOpen = false;
    try {
      await authApi.logout();
    } catch {
      /* die Sitzung ist so oder so vorbei */
    }
    // Auf einem Wandgerät bleibt nach dem Abmelden der Familien-Modus
    // übrig. Nur wo das nicht so ist, geht es zum Anmeldebildschirm.
    try {
      const danach = await authApi.me();
      if ('device' in danach) {
        session.setDevice();
        await goto('/');
        return;
      }
    } catch {
      /* niemand mehr da — normaler Abmeldeweg */
    }
    session.clear();
    await goto('/login');
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (menuOpen = false)} />

<header class="sticky top-0 z-40 border-b border-border bg-card/95 backdrop-blur">
  <div class="mx-auto flex max-w-7xl items-center gap-2 px-3 py-2 sm:px-4">
    <!--
      Der Menüknopf ist die ganze Navigation, solange die Leiste nicht
      hineinpasst. Die Grenze liegt bei lg und nicht bei md: Seit Wetter und
      "Arbeit & Schule" mit in der Leiste stehen, braucht sie rund 1100 Pixel.
      Bei md (768) stand sie über den Rand hinaus und die Seite liess sich
      seitwärts schieben.
    -->
    <button
      class="btn-ghost shrink-0 rounded-xl px-2 lg:hidden"
      onclick={() => (menuOpen = true)}
      aria-label="Menü öffnen"
      aria-expanded={menuOpen}
    >
      <Menu class="h-6 w-6" />
    </button>

    <a href="/" class="flex shrink-0 items-center gap-2 lg:pr-2" aria-label="Startseite">
      <span class="text-xl">🏠</span>
      <span class="hidden text-sm font-semibold sm:inline">Familie</span>
    </a>

    <nav class="hidden items-center gap-1 lg:flex">
      <!--
        Nur "Ansicht anpassen" fehlt hier: Der Weg dorthin steht unten auf der
        Übersicht, direkt bei den Fenstern, die man ordnen will.

        Wetter stand lange ebenfalls nicht hier, weil es über den Wetterblock
        erreichbar war. Das reichte nicht — wer den Ort einstellen will, sucht
        ihn im Menü.
      -->
      {#each nav.filter((i) => i.href !== '/ansicht') as item (item.href)}
        <a
          href={item.href}
          class="touch-target gap-2 rounded-xl px-3 text-sm font-medium transition-colors
            {path === item.href
            ? 'bg-primary text-primary-foreground'
            : 'text-muted-foreground hover:bg-accent hover:text-foreground'}"
        >
          <item.icon class="h-4 w-4" />
          {item.label}
        </a>
      {/each}
    </nav>

    <div class="flex-1"></div>

    <div class="flex shrink-0 items-center gap-1">
      {#if !$connection.online}
        <span class="rounded-full bg-destructive/10 p-1.5 text-destructive" title="Offline">
          <WifiOff class="h-4 w-4" />
        </span>
      {/if}

      <!-- Just my own standing, not the whole table. Am Wandgerät gar nicht. -->
      {#if me && !geraet}
        <a
          href="/rangliste"
          class="flex items-center gap-1.5 rounded-full bg-muted px-3 py-1.5 text-sm transition-colors hover:bg-accent"
          title="Platz {me.rank} · {me.total_points} Punkte · Level {me.level} {me.level_name}"
        >
          {#if myMedal}<span class="leading-none">{myMedal}</span>{/if}
          <span class="font-semibold tabular-nums">{me.total_points}</span>
          <span class="hidden text-xs text-muted-foreground sm:inline">Punkte</span>
        </a>
      {/if}

      {#if user}
        <a
          href="/settings"
          class="btn-ghost rounded-xl px-2"
          aria-label="Profil von {user.name}"
          title={user.name}
        >
          <span class="text-xl leading-none">{user.avatar_emoji}</span>
        </a>
      {:else if geraet}
        <!-- Der Konto-Wechsler: wer das Tablet mit aufs Sofa nimmt, meldet
             sich hier an und bekommt seine persönliche Ansicht. -->
        <a
          href="/login"
          class="flex items-center gap-1.5 rounded-full border border-[color:var(--haarlinie)] px-3 py-1.5 text-sm transition-colors hover:bg-accent"
          title="Als Familienmitglied anmelden"
        >
          <UserRound class="h-4 w-4" />
          <span class="hidden sm:inline">Anmelden</span>
        </a>
      {/if}
    </div>
  </div>
</header>

<!-- Slide-in drawer: the phone menu, and a shortcut sheet on desktop too. -->
{#if menuOpen}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
    onclick={() => (menuOpen = false)}
    role="presentation"
  ></div>

  <div
    class="fixed inset-y-0 left-0 z-50 flex w-[min(20rem,85vw)] animate-slide-in flex-col border-r border-border bg-card shadow-2xl"
  >
    <div class="flex items-center justify-between border-b border-border p-4">
      <span class="flex items-center gap-2 font-semibold">
        <span class="text-xl">🏠</span> Familien Dashboard
      </span>
      <button class="touch-target text-muted-foreground" onclick={() => (menuOpen = false)} aria-label="Menü schließen">
        <X class="h-5 w-5" />
      </button>
    </div>

    {#if user}
      <a
        href="/settings"
        class="flex items-center gap-3 border-b border-border p-4 transition-colors hover:bg-accent"
      >
        <span class="text-3xl">{user.avatar_emoji}</span>
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium">{user.name}</p>
          {#if me}
            <p class="text-xs text-muted-foreground">
              Platz {me.rank} · {me.total_points} Punkte · Level {me.level}
            </p>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
              <div class="h-full bg-primary" style="width: {me.level_progress}%"></div>
            </div>
          {:else}
            <p class="text-xs text-muted-foreground">
              {user.role === 'admin' ? 'Administrator' : 'Familienmitglied'}
            </p>
          {/if}
        </div>
      </a>
    {/if}

    <nav class="flex-1 overflow-y-auto p-2">
      {#each nav as item (item.href)}
        <a
          href={item.href}
          class="flex items-center gap-3 rounded-xl px-3 py-3 text-sm font-medium transition-colors
            {path === item.href ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}"
        >
          <item.icon class="h-5 w-5 shrink-0" />
          {item.label}
        </a>
      {/each}

      <p class="px-3 pb-1 pt-4 text-[11px] uppercase tracking-wider text-muted-foreground">
        Design
      </p>
      <div class="flex gap-1 px-1">
        {#each themes as t (t.value)}
          <button
            class="flex flex-1 flex-col items-center gap-1 rounded-xl py-2.5 text-xs transition-colors
              {$theme === t.value ? 'bg-accent font-medium' : 'hover:bg-accent'}"
            onclick={() => theme.set(t.value)}
          >
            <t.icon class="h-5 w-5" />
            {t.label}
          </button>
        {/each}
      </div>
    </nav>

    <div class="safe-bottom border-t border-border p-2">
      {#if geraet}
        <!-- Im Familien-Modus gibt es nichts abzumelden — aber einen Weg
             in das eigene Konto, wenn das Tablet mit aufs Sofa wandert. -->
        <a
          href="/login"
          class="flex w-full items-center gap-3 rounded-xl px-3 py-3 text-sm transition-colors hover:bg-accent"
          onclick={() => (menuOpen = false)}
        >
          <UserRound class="h-5 w-5" />
          Als Familienmitglied anmelden
        </a>
      {:else}
        <button
          class="flex w-full items-center gap-3 rounded-xl px-3 py-3 text-sm text-destructive transition-colors hover:bg-accent"
          onclick={signOut}
        >
          <LogOut class="h-5 w-5" />
          Abmelden
        </button>
      {/if}
    </div>
  </div>
{/if}
