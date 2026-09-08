<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Check, KeyRound, LayoutGrid, Monitor, Moon, Palette, Sun, User as UserIcon,
  } from 'lucide-svelte';
  import { ApiError, authApi } from '$lib/api';
  import { session, theme, type Theme } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';

  let currentPin = $state('');
  let newPin = $state('');
  let confirmPin = $state('');
  let pinError = $state('');
  let pinSaved = $state(false);
  let saving = $state(false);

  const themes: { value: Theme; label: string; hint: string; icon: typeof Sun }[] = [
    { value: 'light', label: 'Hell', hint: 'Immer hell', icon: Sun },
    { value: 'dark', label: 'Dunkel', hint: 'Immer dunkel', icon: Moon },
    { value: 'system', label: 'System', hint: 'Folgt dem Gerät', icon: Monitor },
  ];

  const user = $derived($session.user);

  // ---- Profil ----
  const emojis = [
    '👨', '👩', '🧒', '👦', '👧', '👴', '👵', '🧑', '👨‍🦰', '👩‍🦰',
    '🐶', '🐱', '🦊', '🐼', '🦄', '⭐', '🚀', '🎸', '⚽', '🌻',
  ];
  const colors = [
    '#3b82f6', '#ec4899', '#f59e0b', '#10b981', '#8b5cf6',
    '#ef4444', '#14b8a6', '#f97316', '#6366f1', '#64748b',
  ];

  let profileName = $state('');
  let profileEmoji = $state('');
  let profileColor = $state('');
  let profileError = $state('');
  let profileSaved = $state(false);
  let savingProfile = $state(false);

  const profileChanged = $derived(
    !!user &&
      (profileName.trim() !== user.name ||
        profileEmoji !== user.avatar_emoji ||
        profileColor !== user.color),
  );

  // Seed the form once the session has resolved.
  $effect(() => {
    if (user && profileName === '') {
      profileName = user.name;
      profileEmoji = user.avatar_emoji;
      profileColor = user.color;
    }
  });

  async function saveProfile(event: SubmitEvent) {
    event.preventDefault();
    if (!profileChanged || savingProfile) return;
    profileError = '';
    profileSaved = false;
    savingProfile = true;
    try {
      const updated = await authApi.updateProfile({
        name: profileName.trim(),
        avatar_emoji: profileEmoji,
        color: profileColor,
      });
      session.set(updated);
      // The leaderboard shows names and avatars, so refresh it too.
      void board.refresh();
      profileSaved = true;
      setTimeout(() => (profileSaved = false), 3000);
    } catch (e) {
      profileError = e instanceof ApiError ? e.message : 'Profil konnte nicht gespeichert werden';
    } finally {
      savingProfile = false;
    }
  }

  async function changePin(event: SubmitEvent) {
    event.preventDefault();
    pinError = '';
    pinSaved = false;

    if (!/^\d{4}$/.test(newPin)) {
      pinError = 'Die neue PIN muss aus genau 4 Ziffern bestehen';
      return;
    }
    if (newPin !== confirmPin) {
      pinError = 'Die beiden neuen PINs stimmen nicht überein';
      return;
    }
    if (newPin === currentPin) {
      pinError = 'Die neue PIN entspricht der alten';
      return;
    }

    saving = true;
    try {
      await authApi.changePin(currentPin, newPin);
      pinSaved = true;
      currentPin = newPin = confirmPin = '';
      // Reflect that the default PIN warning can go away.
      const me = await authApi.me();
      session.set(me);
    } catch (e) {
      pinError = e instanceof ApiError ? e.message : 'Konnte PIN nicht ändern';
    } finally {
      saving = false;
    }
  }

  onMount(() => theme.init());
</script>

<svelte:head><title>Einstellungen · Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-2xl px-4 py-6">
  <h1 class="mb-6 text-2xl font-semibold">Einstellungen</h1>

  {#if user}
    <section class="card mb-4 p-5">
      <h2 class="mb-4 flex items-center gap-2 font-semibold">
        <UserIcon class="h-5 w-5" /> Mein Profil
      </h2>

      <form class="space-y-4" onsubmit={saveProfile}>
        <div class="flex items-center gap-4">
          <span
            class="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl text-4xl"
            style="background-color: {profileColor}22"
          >
            {profileEmoji}
          </span>
          <label class="min-w-0 flex-1 text-sm">
            Name
            <input class="input mt-1" bind:value={profileName} maxlength="40" required />
          </label>
        </div>

        <div class="text-sm">
          Avatar
          <div class="mt-1.5 grid grid-cols-10 gap-1">
            {#each emojis as emoji}
              <button
                type="button"
                class="flex h-10 items-center justify-center rounded-lg text-xl transition-colors
                  {profileEmoji === emoji ? 'bg-primary/15 ring-2 ring-primary' : 'hover:bg-accent'}"
                onclick={() => (profileEmoji = emoji)}
                aria-label="Avatar {emoji}"
                aria-pressed={profileEmoji === emoji}
              >
                {emoji}
              </button>
            {/each}
          </div>
        </div>

        <div class="text-sm">
          Farbe
          <div class="mt-1.5 flex flex-wrap gap-2">
            {#each colors as color}
              <button
                type="button"
                class="h-9 w-9 rounded-full transition-transform {profileColor === color
                  ? 'scale-110 ring-2 ring-offset-2 ring-offset-card'
                  : ''}"
                style="background-color: {color}; --tw-ring-color: {color}"
                aria-label="Farbe {color}"
                aria-pressed={profileColor === color}
                onclick={() => (profileColor = color)}
              ></button>
            {/each}
          </div>
        </div>

        {#if profileError}
          <p class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {profileError}
          </p>
        {/if}
        {#if profileSaved}
          <p class="flex items-center gap-2 rounded-lg bg-success/10 px-3 py-2 text-sm text-success">
            <Check class="h-4 w-4" /> Profil gespeichert
          </p>
        {/if}

        <button class="btn-primary w-full" disabled={!profileChanged || savingProfile}>
          {savingProfile ? 'Wird gespeichert…' : 'Profil speichern'}
        </button>
      </form>

      <p class="mt-3 text-xs text-muted-foreground">
        Rolle: {user.role === 'admin' ? 'Administrator' : 'Familienmitglied'} – die ändert
        nur ein Administrator.
      </p>
    </section>
  {/if}

  <a
    href="/ansicht"
    class="card mb-4 flex items-center gap-3 p-5 transition-colors hover:bg-accent"
  >
    <LayoutGrid class="h-5 w-5 shrink-0" />
    <span class="min-w-0 flex-1">
      <span class="block font-semibold">Ansicht anpassen</span>
      <span class="block text-sm text-muted-foreground">
        Welche Fenster die Übersicht zeigt und in welcher Reihenfolge
      </span>
    </span>
    <span class="shrink-0 text-muted-foreground">→</span>
  </a>

  <section class="card mb-4 p-5">
    <h2 class="mb-4 flex items-center gap-2 font-semibold">
      <Palette class="h-5 w-5" /> Darstellung
    </h2>
    <div class="grid grid-cols-3 gap-2">
      {#each themes as option}
        <button
          class="flex flex-col items-center gap-2 rounded-lg border p-4 transition-colors
            {$theme === option.value
            ? 'border-primary bg-primary/5'
            : 'border-border hover:bg-accent'}"
          onclick={() => theme.set(option.value)}
        >
          <option.icon class="h-6 w-6" />
          <span class="text-sm font-medium">{option.label}</span>
          <span class="text-[11px] text-muted-foreground">{option.hint}</span>
        </button>
      {/each}
    </div>
  </section>

  <section class="card p-5">
    <h2 class="mb-4 flex items-center gap-2 font-semibold">
      <KeyRound class="h-5 w-5" /> PIN ändern
    </h2>

    {#if user?.pin_is_default}
      <p class="mb-4 rounded-lg bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-400">
        Du benutzt noch die Standard-PIN <strong>1234</strong>. Bitte ändere sie.
      </p>
    {/if}

    <form class="space-y-3" onsubmit={changePin}>
      <label class="block text-sm">
        Aktuelle PIN
        <input
          class="input mt-1 tracking-[0.4em]"
          type="password"
          inputmode="numeric"
          maxlength="4"
          autocomplete="current-password"
          bind:value={currentPin}
        />
      </label>
      <label class="block text-sm">
        Neue PIN
        <input
          class="input mt-1 tracking-[0.4em]"
          type="password"
          inputmode="numeric"
          maxlength="4"
          autocomplete="new-password"
          bind:value={newPin}
        />
      </label>
      <label class="block text-sm">
        Neue PIN wiederholen
        <input
          class="input mt-1 tracking-[0.4em]"
          type="password"
          inputmode="numeric"
          maxlength="4"
          autocomplete="new-password"
          bind:value={confirmPin}
        />
      </label>

      {#if pinError}
        <p class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{pinError}</p>
      {/if}
      {#if pinSaved}
        <p class="flex items-center gap-2 rounded-lg bg-success/10 px-3 py-2 text-sm text-success">
          <Check class="h-4 w-4" /> PIN geändert
        </p>
      {/if}

      <button
        class="btn-primary w-full"
        disabled={saving || !currentPin || !newPin || !confirmPin}
      >
        PIN speichern
      </button>
    </form>
  </section>

  <p class="mt-6 text-center text-xs text-muted-foreground">
    Familien Dashboard · läuft lokal im Heimnetz
  </p>
</div>
