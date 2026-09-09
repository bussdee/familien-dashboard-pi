<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { ApiError, authApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { Delete, LoaderCircle, TriangleAlert } from 'lucide-svelte';
  import type { User } from '$lib/types';

  let users = $state<User[]>([]);
  let selected = $state<User | null>(null);
  let pin = $state('');
  let error = $state('');
  let loading = $state(false);
  let loadingRoster = $state(true);

  onMount(async () => {
    try {
      users = await authApi.roster();
      if (users.length === 1) selected = users[0];
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Server nicht erreichbar';
    } finally {
      loadingRoster = false;
    }

    // Already signed in? Go straight to the dashboard.
    try {
      const me = await authApi.me();
      // Ein Wandgerät ist zwar "angemeldet", aber niemand persönlich.
      // Weiterleiten wäre hier falsch: wer hier landet, will sich gerade
      // anmelden — sonst käme er nie in sein eigenes Konto.
      if ('device' in me) {
        session.setDevice();
        return;
      }
      session.set(me);
      await goto('/');
    } catch {
      session.clear();
    }
  });

  function pick(user: User) {
    selected = user;
    pin = '';
    error = '';
  }

  function press(digit: string) {
    if (loading || pin.length >= 4) return;
    error = '';
    pin += digit;
    if (navigator.vibrate) navigator.vibrate(8);
    if (pin.length === 4) void submit();
  }

  function backspace() {
    pin = pin.slice(0, -1);
    error = '';
  }

  async function submit() {
    if (!selected || pin.length !== 4 || loading) return;
    loading = true;
    try {
      const { user } = await authApi.login(selected.id, pin);
      session.set(user);
      await goto('/');
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Anmeldung fehlgeschlagen';
      pin = '';
      if (navigator.vibrate) navigator.vibrate([40, 60, 40]);
    } finally {
      loading = false;
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (!selected) return;
    if (event.key >= '0' && event.key <= '9') press(event.key);
    else if (event.key === 'Backspace') backspace();
    else if (event.key === 'Escape') selected = null;
  }
</script>

<svelte:window onkeydown={onKeydown} />

<svelte:head><title>Anmelden · Familien Dashboard</title></svelte:head>

<div class="flex min-h-screen items-center justify-center p-4">
  <div class="w-full max-w-sm">
    <div class="mb-8 text-center">
      <div class="mb-2 text-5xl">🏠</div>
      <h1 class="text-2xl font-semibold">Familien Dashboard</h1>
      <p class="text-sm text-muted-foreground">
        {selected ? `PIN für ${selected.name} eingeben` : 'Wer bist du?'}
      </p>
    </div>

    {#if loadingRoster}
      <div class="flex justify-center py-10 text-muted-foreground">
        <LoaderCircle class="h-6 w-6 animate-spin" />
      </div>
    {:else if users.length === 0}
      <div class="card p-6 text-center">
        <TriangleAlert class="mx-auto mb-2 h-8 w-8 text-destructive" />
        <p class="text-sm">{error || 'Keine Benutzer gefunden.'}</p>
        <button class="btn-outline mt-4 w-full" onclick={() => location.reload()}>
          Erneut versuchen
        </button>
      </div>
    {:else if !selected}
      <div class="grid grid-cols-2 gap-3">
        {#each users as user (user.id)}
          <button
            class="card flex flex-col items-center gap-2 p-5 transition-transform active:scale-95"
            style="border-color: {user.color}55"
            onclick={() => pick(user)}
          >
            <span class="text-4xl">{user.avatar_emoji}</span>
            <span class="font-medium">{user.name}</span>
            {#if user.pin_is_default}
              <span class="rounded-full bg-amber-500/15 px-2 py-0.5 text-[11px] text-amber-600 dark:text-amber-400">
                Standard-PIN
              </span>
            {/if}
          </button>
        {/each}
      </div>
    {:else}
      <div class="card p-6">
        <button
          class="mb-5 flex w-full items-center justify-center gap-3"
          onclick={() => (selected = null)}
        >
          <span class="text-4xl">{selected.avatar_emoji}</span>
          <span>
            <span class="block font-medium">{selected.name}</span>
            <span class="block text-xs text-muted-foreground">wechseln</span>
          </span>
        </button>

        <div class="mb-5 flex justify-center gap-3" aria-label="PIN-Eingabe">
          {#each [0, 1, 2, 3] as i}
            <div
              class="h-4 w-4 rounded-full border-2 transition-colors
                {i < pin.length ? 'border-primary bg-primary' : 'border-border'}"
            ></div>
          {/each}
        </div>

        {#if error}
          <p class="mb-4 rounded-lg bg-destructive/10 px-3 py-2 text-center text-sm text-destructive">
            {error}
          </p>
        {/if}

        <div class="grid grid-cols-3 gap-2">
          {#each ['1', '2', '3', '4', '5', '6', '7', '8', '9'] as digit}
            <button
              class="btn-outline h-14 text-xl font-medium"
              onclick={() => press(digit)}
              disabled={loading}
            >
              {digit}
            </button>
          {/each}
          <div></div>
          <button
            class="btn-outline h-14 text-xl font-medium"
            onclick={() => press('0')}
            disabled={loading}
          >
            0
          </button>
          <button
            class="btn-outline h-14"
            onclick={backspace}
            disabled={loading || pin.length === 0}
            aria-label="Löschen"
          >
            <Delete class="h-5 w-5" />
          </button>
        </div>

        {#if loading}
          <div class="mt-4 flex justify-center text-muted-foreground">
            <LoaderCircle class="h-5 w-5 animate-spin" />
          </div>
        {/if}
      </div>
    {/if}

    <p class="mt-6 text-center text-xs text-muted-foreground">
      Läuft lokal im Heimnetz · keine Cloud
    </p>
    <p class="mt-2 flex items-center justify-center gap-2 text-[11px] text-muted-foreground">
      <a
        href="https://familienfabrik.at"
        target="_blank"
        rel="noopener noreferrer"
        class="transition-colors hover:text-foreground hover:underline"
      >
        familienfabrik.at
      </a>
    </p>
  </div>
</div>
