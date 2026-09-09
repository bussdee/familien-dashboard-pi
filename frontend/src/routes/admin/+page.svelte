<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    Check, ChevronDown, ChevronUp, Database, Download, HardDriveDownload,
    ChevronRight, CornerLeftUp, Folder, MonitorSmartphone, Music, Plus, RefreshCw,
    RotateCcw, Shield, Sparkles, Trash2, TriangleAlert, Undo2, Users, X, Zap,
  } from 'lucide-svelte';
  import { formatDistanceToNow, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { ApiError, adminApi, authApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import type {
    Activity, BackupFile, DeviceTarget, MusicDirListing, MusicStatus, User,
  } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  // ---- Wandgerät ----
  let istWandgeraet = $state(false);
  let geraetFehler = $state('');

  async function wandgeraetAn() {
    geraetFehler = '';
    busy = true;
    try {
      await authApi.enableDevice();
      istWandgeraet = true;
      // Der Server hat die persönliche Sitzung beendet. Auf der
      // Verwaltungsseite zu bleiben, ginge nicht mehr — und wäre auch falsch:
      // Das Gerät ist ab jetzt das Familiengerät, also gehört es auf die
      // Übersicht.
      session.setDevice();
      await goto('/');
    } catch (e) {
      geraetFehler = e instanceof ApiError ? e.message : 'Konnte nicht einrichten';
      busy = false;
    }
  }

  async function wandgeraetAus() {
    geraetFehler = '';
    busy = true;
    try {
      await authApi.disableDevice();
      istWandgeraet = false;
    } catch (e) {
      geraetFehler = e instanceof ApiError ? e.message : 'Konnte nicht aufheben';
    } finally {
      busy = false;
    }
  }

  let users = $state<User[]>([]);
  let backups = $state<BackupFile[]>([]);
  let error = $state('');
  let notice = $state('');
  let loading = $state(true);
  let busy = $state(false);
  let showForm = $state(false);
  let editing = $state<User | null>(null);

  const emojis = ['👨', '👩', '🧒', '👦', '👧', '👴', '👵', '🧑', '🐶', '🐱', '⭐', '🚀'];
  const colors = ['#3b82f6', '#ec4899', '#f59e0b', '#10b981', '#8b5cf6', '#ef4444', '#14b8a6', '#64748b'];

  let draft = $state({
    name: '', color: colors[0], pin: '', role: 'member', avatar_emoji: '🧑',
    in_rotation: true,
  });

  const size = (bytes: number) =>
    bytes > 1_048_576
      ? `${(bytes / 1_048_576).toFixed(1)} MB`
      : `${Math.max(1, Math.round(bytes / 1024))} KB`;

  // ---- Punkte ----
  let history = $state<Activity[]>([]);
  let historyUser = $state(0);
  let pointsError = $state('');
  let adjustUser = $state(0);
  let adjustAmount = $state(-5);
  let adjustNote = $state('');

  const sourceLabel: Record<string, string> = {
    chore: 'Aufgabe',
    shopping: 'Einkauf',
    bonus: 'Manuell',
  };

  const relative = (iso: string) =>
    formatDistanceToNow(parseISO(iso), { addSuffix: true, locale: de });

  async function loadHistory() {
    try {
      history = await adminApi.pointHistory(historyUser || undefined);
    } catch (e) {
      pointsError = e instanceof ApiError ? e.message : 'Verlauf konnte nicht geladen werden';
    }
  }

  async function revoke(entry: Activity) {
    const ok = await confirmAction({
      title: `${entry.points} Punkte zurücknehmen?`,
      message:
        entry.source === 'chore'
          ? `„${entry.note}“ wird dadurch wieder fällig.`
          : `Eintrag: „${entry.note}“`,
      confirmLabel: 'Zurücknehmen',
    });
    if (!ok) return;
    pointsError = '';
    try {
      await adminApi.revokePoints(entry.id);
      await Promise.all([loadHistory(), board.refresh()]);
      notice = 'Eintrag zurückgenommen';
    } catch (e) {
      pointsError = e instanceof ApiError ? e.message : 'Zurücknehmen fehlgeschlagen';
    }
  }

  async function adjust(event: SubmitEvent) {
    event.preventDefault();
    if (!adjustUser || adjustAmount === 0) return;
    pointsError = '';
    busy = true;
    try {
      await adminApi.adjustPoints(adjustUser, adjustAmount, adjustNote.trim());
      adjustNote = '';
      await Promise.all([loadHistory(), board.refresh()]);
      notice = adjustAmount > 0 ? 'Punkte gutgeschrieben' : 'Punkte abgezogen';
    } catch (e) {
      pointsError = e instanceof ApiError ? e.message : 'Buchung fehlgeschlagen';
    } finally {
      busy = false;
    }
  }

  async function resetPoints(userId: number, label: string) {
    const ok = await confirmAction({
      title: `Punkte von ${label} zurücksetzen?`,
      message: 'Der gesamte Verlauf wird gelöscht. Das lässt sich nicht rückgängig machen.',
      confirmLabel: 'Zurücksetzen',
    });
    if (!ok) return;
    pointsError = '';
    try {
      const { removed } = await adminApi.resetPoints(userId);
      await Promise.all([loadHistory(), board.refresh()]);
      notice = `${removed} Einträge entfernt`;
    } catch (e) {
      pointsError = e instanceof ApiError ? e.message : 'Zurücksetzen fehlgeschlagen';
    }
  }

  // ---- Geräte ----
  let devices = $state<DeviceTarget[]>([]);
  let showDeviceForm = $state(false);
  let editingDevice = $state<DeviceTarget | null>(null);
  let deviceError = $state('');
  let testResult = $state<{ status: string; latency_ms: number; error: string } | null>(null);
  let testing = $state(false);

  function emptyDevice() {
    return {
      name: '',
      type: 'http' as 'http' | 'tcp',
      url: '',
      link: '',
      host: '',
      port: 0,
      expect_status: 0,
      icon: '',
      enabled: true,
    };
  }

  let deviceDraft = $state(emptyDevice());

  /**
   * Punkt 8 aus dem Plan: Die Prüf-Adresse und der Link zur Oberfläche sind
   * zwei verschiedene Felder — und genau deshalb geraten sie auseinander. Wer
   * beim Umzug ins neue Netz nur die Prüfung anpasst, bekommt eine grüne
   * Kachel, die beim Antippen ins Leere führt. Die Prüfung sagt ja
   * "erreichbar", also sieht alles richtig aus.
   *
   * Das ist kein Fehler und wird deshalb auch nicht abgelehnt: Ein Dienst
   * darf hinter einem Namen liegen und unter einer IP geprüft werden. Nur
   * selten ist es Absicht, also ein Hinweis.
   */
  function hostVon(adresse: string): string {
    try {
      return new URL(adresse.trim()).host.toLowerCase();
    } catch {
      return '';
    }
  }

  const hostWarnung = $derived.by(() => {
    if (deviceDraft.type !== 'http') return null;
    const pruefung = hostVon(deviceDraft.url);
    const ziel = hostVon(deviceDraft.link);
    if (!pruefung || !ziel || pruefung === ziel) return null;
    return { pruefung, ziel };
  });

  function startNewDevice() {
    editingDevice = null;
    deviceDraft = emptyDevice();
    deviceError = '';
    testResult = null;
    showDeviceForm = true;
  }

  function startEditDevice(device: DeviceTarget) {
    editingDevice = device;
    deviceDraft = {
      name: device.name,
      type: device.type,
      url: device.url,
      link: device.link,
      host: device.host,
      port: device.port,
      expect_status: device.expect_status,
      icon: device.icon,
      enabled: device.enabled,
    };
    deviceError = '';
    testResult = null;
    showDeviceForm = true;
  }

  async function testDevice() {
    testing = true;
    deviceError = '';
    testResult = null;
    try {
      testResult = await adminApi.testDevice(deviceDraft);
    } catch (e) {
      deviceError = e instanceof ApiError ? e.message : 'Test fehlgeschlagen';
    } finally {
      testing = false;
    }
  }

  async function saveDevice(event: SubmitEvent) {
    event.preventDefault();
    deviceError = '';
    busy = true;
    try {
      if (editingDevice) await adminApi.updateDevice(editingDevice.id, deviceDraft);
      else await adminApi.createDevice(deviceDraft);
      showDeviceForm = false;
      editingDevice = null;
      devices = await adminApi.listDevices();
      notice = 'Gerät gespeichert';
    } catch (e) {
      deviceError = e instanceof ApiError ? e.message : 'Speichern fehlgeschlagen';
    } finally {
      busy = false;
    }
  }

  async function toggleDevice(device: DeviceTarget) {
    try {
      await adminApi.updateDevice(device.id, { ...device, enabled: !device.enabled });
      devices = await adminApi.listDevices();
    } catch (e) {
      deviceError = e instanceof ApiError ? e.message : 'Konnte nicht umschalten';
    }
  }

  async function moveDevice(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= devices.length) return;
    const next = [...devices];
    [next[index], next[target]] = [next[target], next[index]];
    devices = next;
    try {
      await adminApi.reorderDevices(next.map((d) => d.id));
    } catch {
      devices = await adminApi.listDevices();
    }
  }

  async function removeDevice(device: DeviceTarget) {
    const ok = await confirmAction({ title: `Gerät „${device.name}“ entfernen?` });
    if (!ok) return;
    try {
      await adminApi.deleteDevice(device.id);
      devices = await adminApi.listDevices();
      notice = `${device.name} entfernt`;
    } catch (e) {
      deviceError = e instanceof ApiError ? e.message : 'Löschen fehlgeschlagen';
    }
  }

  async function load() {
    try {
      [users, backups, devices, history] = await Promise.all([
        adminApi.listUsers(),
        adminApi.listBackups(),
        adminApi.listDevices(),
        adminApi.pointHistory(),
      ]);
      // Die Musik darf nachkommen: Ist die Platte abgemeldet, soll das nicht
      // die ganze Verwaltungsseite aufhalten.
      void ladeMusik();
      if (!board.loaded) void board.refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Laden fehlgeschlagen';
    } finally {
      loading = false;
    }
  }

  function startNew() {
    editing = null;
    draft = {
      name: '', color: colors[0], pin: '', role: 'member', avatar_emoji: '🧑',
      in_rotation: true,
    };
    showForm = true;
  }

  function startEdit(user: User) {
    editing = user;
    draft = {
      name: user.name,
      color: user.color,
      pin: '',
      role: user.role,
      avatar_emoji: user.avatar_emoji,
      in_rotation: user.in_rotation,
    };
    showForm = true;
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    notice = '';
    busy = true;
    try {
      if (editing) {
        // An empty PIN field means "leave the PIN alone".
        const payload: Record<string, string | boolean> = {
          name: draft.name,
          color: draft.color,
          role: draft.role,
          avatar_emoji: draft.avatar_emoji,
          in_rotation: draft.in_rotation,
        };
        if (draft.pin) payload.pin = draft.pin;
        await adminApi.updateUser(editing.id, payload);
        notice = `${draft.name} aktualisiert`;
      } else {
        await adminApi.createUser(draft);
        notice = `${draft.name} angelegt`;
      }
      showForm = false;
      editing = null;
      users = await adminApi.listUsers();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Speichern fehlgeschlagen';
    } finally {
      busy = false;
    }
  }

  async function remove(user: User) {
    const ok = await confirmAction({
      title: `${user.name} löschen?`,
      message: 'Punkte, Notizen und Links dieser Person gehen verloren.',
    });
    if (!ok) return;
    error = '';
    try {
      await adminApi.deleteUser(user.id);
      users = await adminApi.listUsers();
      notice = `${user.name} gelöscht`;
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Löschen fehlgeschlagen';
    }
  }

  // ---- Musik ----
  let musik = $state<MusicStatus | null>(null);
  let musikOrdner = $state<MusicDirListing | null>(null);
  let musikFehler = $state('');
  let musikOffen = $state(false);
  let musikTimer: ReturnType<typeof setInterval> | null = null;

  const musikPfadTeile = $derived.by(() => {
    const pfad = musikOrdner?.path ?? '';
    if (!pfad) return [];
    const teile = pfad.split('/');
    return teile.map((name, i) => ({ name, path: teile.slice(0, i + 1).join('/') }));
  });

  async function ladeMusik() {
    try {
      musik = await adminApi.musicStatus();
    } catch {
      musik = null;
    }
  }

  async function musikBlaettern(pfad: string) {
    musikFehler = '';
    try {
      musikOrdner = await adminApi.musicFolders(pfad);
    } catch (e) {
      musikFehler = e instanceof ApiError ? e.message : 'Ordner nicht lesbar';
      musikOrdner = null;
    }
  }

  async function musikOrdnerWaehlen(pfad: string) {
    const ok = await confirmAction({
      title: pfad ? `„${pfad}" als Musikordner setzen?` : 'Den ganzen Ordner verwenden?',
      message:
        'Der bisherige Index wird verworfen und neu aufgebaut. Bei einer grossen ' +
        'Sammlung dauert das einige Minuten — das Dashboard bleibt währenddessen ' +
        'bedienbar.',
      confirmLabel: 'Setzen und einlesen',
    });
    if (!ok) return;

    musikFehler = '';
    busy = true;
    try {
      await adminApi.setMusicDir(pfad);
      notice = 'Musikordner gesetzt, wird eingelesen';
      await Promise.all([ladeMusik(), musikBlaettern(musikOrdner?.path ?? '')]);
    } catch (e) {
      musikFehler = e instanceof ApiError ? e.message : 'Konnte nicht gesetzt werden';
    } finally {
      busy = false;
    }
  }

  async function musikNeuEinlesen() {
    musikFehler = '';
    try {
      await adminApi.rescanMusic();
      await ladeMusik();
    } catch (e) {
      musikFehler = e instanceof ApiError ? e.message : 'Einlesen fehlgeschlagen';
    }
  }

  async function runBackup() {
    busy = true;
    error = '';
    try {
      const result = await adminApi.runBackup();
      notice = `Backup erstellt: ${result.database}`;
      backups = await adminApi.listBackups();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Backup fehlgeschlagen';
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    if ($session.ready && $session.user?.role !== 'admin') {
      void goto('/');
      return;
    }
    // Der Server weiß, ob auf DIESEM Gerät der Familien-Modus eingerichtet
    // ist — der Keks liegt hier, nicht in der Datenbank.
    istWandgeraet = $session.user?.device_mode === true;
    void load();

    // Läuft gerade ein Durchlauf, wächst die Zahl im Hintergrund.
    musikTimer = setInterval(() => {
      if (musik?.progress?.running) void ladeMusik();
    }, 4000);
    return () => {
      if (musikTimer) clearInterval(musikTimer);
    };
  });
</script>

<svelte:head><title>Verwaltung · Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-3xl px-4 py-6">
  <h1 class="mb-6 flex items-center gap-2 text-2xl font-semibold">
    <Shield class="h-6 w-6" /> Verwaltung
  </h1>

  {#if error}
    <p class="mb-4 rounded-lg bg-destructive/10 px-4 py-2 text-sm text-destructive">{error}</p>
  {/if}
  {#if notice}
    <p class="mb-4 flex items-center gap-2 rounded-lg bg-success/10 px-4 py-2 text-sm text-success">
      <Check class="h-4 w-4" />
      {notice}
    </p>
  {/if}

  <section class="card mb-4 p-5">
    <header class="mb-4 flex items-center justify-between">
      <h2 class="flex items-center gap-2 font-semibold">
        <Users class="h-5 w-5" /> Familienmitglieder
      </h2>
      <button class="btn-primary px-3" onclick={() => (showForm ? (showForm = false) : startNew())}>
        {#if showForm}<X class="h-5 w-5" />{:else}<Plus class="h-5 w-5" />{/if}
      </button>
    </header>

    {#if showForm}
      <form class="mb-4 space-y-3 rounded-lg border border-border p-4" onsubmit={save}>
        <label class="block text-sm">
          Name
          <input class="input mt-1" bind:value={draft.name} maxlength="40" required />
        </label>

        <div class="text-sm">
          Avatar
          <div class="mt-1 flex flex-wrap gap-1">
            {#each emojis as emoji}
              <button
                type="button"
                class="h-10 w-10 rounded-lg text-xl transition-colors
                  {draft.avatar_emoji === emoji ? 'bg-primary/15 ring-2 ring-primary' : 'hover:bg-accent'}"
                onclick={() => (draft.avatar_emoji = emoji)}
              >
                {emoji}
              </button>
            {/each}
          </div>
        </div>

        <div class="text-sm">
          Farbe
          <div class="mt-1 flex flex-wrap gap-2">
            {#each colors as color}
              <button
                type="button"
                class="h-8 w-8 rounded-full transition-transform {draft.color === color
                  ? 'scale-110 ring-2 ring-offset-2 ring-offset-card'
                  : ''}"
                style="background-color: {color}; --tw-ring-color: {color}"
                aria-label="Farbe {color}"
                onclick={() => (draft.color = color)}
              ></button>
            {/each}
          </div>
        </div>

        <label class="block text-sm">
          PIN (4 Ziffern){editing ? ' — leer lassen zum Beibehalten' : ''}
          <input
            class="input mt-1 tracking-[0.4em]"
            type="password"
            inputmode="numeric"
            maxlength="4"
            bind:value={draft.pin}
            required={!editing}
          />
        </label>

        <label class="block text-sm">
          Rolle
          <select class="input mt-1" bind:value={draft.role}>
            <option value="member">Familienmitglied</option>
            <option value="admin">Administrator</option>
          </select>
        </label>

        <label class="flex items-start gap-2 text-sm">
          <input
            type="checkbox"
            class="mt-0.5 h-4 w-4 shrink-0 rounded"
            bind:checked={draft.in_rotation}
          />
          <span>
            Nimmt an der Reihum-Verteilung teil
            <span class="mt-0.5 block text-xs text-muted-foreground">
              Aus dem Häkchen genommen, steht diese Person bei „reihum" nicht
              mehr im Plan. Feste Zuständigkeiten und „alle" bleiben davon
              unberührt.
            </span>
          </span>
        </label>

        <button class="btn-primary w-full" disabled={busy || !draft.name.trim()}>
          {editing ? 'Änderungen speichern' : 'Anlegen'}
        </button>
      </form>
    {/if}

    {#if loading}
      <p class="py-6 text-center text-sm text-muted-foreground">Lade…</p>
    {:else}
      <ul class="space-y-2">
        {#each users as user (user.id)}
          <li class="flex items-center gap-3 rounded-lg bg-muted/30 p-3">
            <span
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-xl"
              style="background-color: {user.color}22"
            >
              {user.avatar_emoji}
            </span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{user.name}</p>
              <p class="text-xs text-muted-foreground">
                {user.role === 'admin' ? 'Administrator' : 'Familienmitglied'}
                {#if !user.in_rotation}· nicht reihum{/if}
                {#if user.pin_is_default}· <span class="text-amber-600 dark:text-amber-400">Standard-PIN</span>{/if}
              </p>
            </div>
            <button class="btn-ghost px-3 text-sm" onclick={() => startEdit(user)}>Bearbeiten</button>
            <button
              class="touch-target shrink-0 text-muted-foreground hover:text-destructive"
              onclick={() => remove(user)}
              aria-label="{user.name} löschen"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="card mb-4 p-5">
    <header class="mb-4">
      <h2 class="flex items-center gap-2 font-semibold">
        <Sparkles class="h-5 w-5" /> Punkte
      </h2>
      <p class="text-sm text-muted-foreground">
        Punktestände korrigieren und einzelne Einträge zurücknehmen.
      </p>
    </header>

    {#if pointsError}
      <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {pointsError}
      </p>
    {/if}

    <!-- Aktueller Stand mit Reset je Person -->
    <ul class="mb-4 space-y-2">
      {#each board.podium as score (score.id)}
        <li class="flex items-center gap-3 rounded-xl bg-muted/30 p-3">
          <span class="text-xl">{score.avatar_emoji}</span>
          <span class="min-w-0 flex-1 truncate text-sm font-medium">{score.name}</span>
          <span class="shrink-0 text-sm font-semibold tabular-nums">
            {score.total_points} P
          </span>
          <button
            class="btn-ghost shrink-0 px-2 text-xs text-muted-foreground hover:text-destructive"
            onclick={() => resetPoints(score.id, score.name)}
            disabled={score.total_points === 0 && score.activities === 0}
            title="Punktestand von {score.name} zurücksetzen"
          >
            <RotateCcw class="h-4 w-4" />
          </button>
        </li>
      {/each}
    </ul>

    <button
      class="btn-outline mb-4 w-full text-sm text-destructive"
      onclick={() => resetPoints(0, 'der ganzen Familie')}
    >
      <RotateCcw class="h-4 w-4" /> Alle Punkte zurücksetzen
    </button>

    <!-- Manuelle Buchung -->
    <form class="mb-4 space-y-2 rounded-xl border border-border p-3" onsubmit={adjust}>
      <p class="text-sm font-medium">Punkte gutschreiben oder abziehen</p>
      <div class="grid gap-2 sm:grid-cols-[1fr_auto]">
        <select class="input" bind:value={adjustUser} aria-label="Benutzer">
          <option value={0}>Wer?</option>
          {#each users as user (user.id)}
            <option value={user.id}>{user.avatar_emoji} {user.name}</option>
          {/each}
        </select>
        <input
          class="input sm:w-28"
          type="number"
          bind:value={adjustAmount}
          min="-10000"
          max="10000"
          aria-label="Punkte (negativ zum Abziehen)"
        />
      </div>
      <input class="input" placeholder="Grund (optional)" bind:value={adjustNote} maxlength="80" />
      <p class="text-xs text-muted-foreground">
        Negative Zahl zieht ab, positive schreibt gut.
      </p>
      <button class="btn-primary w-full text-sm" disabled={busy || !adjustUser || adjustAmount === 0}>
        {adjustAmount < 0 ? `${Math.abs(adjustAmount)} Punkte abziehen` : `${adjustAmount} Punkte gutschreiben`}
      </button>
    </form>

    <!-- Verlauf mit Rücknahme -->
    <div class="mb-2 flex items-center justify-between gap-2">
      <p class="text-sm font-medium">Verlauf</p>
      <select
        class="input w-auto py-1.5 text-sm"
        bind:value={historyUser}
        onchange={loadHistory}
        aria-label="Nach Benutzer filtern"
      >
        <option value={0}>Alle</option>
        {#each users as user (user.id)}
          <option value={user.id}>{user.name}</option>
        {/each}
      </select>
    </div>

    {#if history.length === 0}
      <p class="py-6 text-center text-sm text-muted-foreground">
        Noch keine Punkte vergeben.
      </p>
    {:else}
      <ul class="scrollbar-thin max-h-80 space-y-1.5 overflow-y-auto pr-1">
        {#each history as entry (entry.id)}
          <li class="flex items-center gap-2.5 rounded-lg bg-muted/30 p-2.5">
            <span class="text-lg">{entry.user_emoji}</span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm">
                <span class="font-medium">{entry.user_name}</span>
                <span class="text-muted-foreground">
                  · {sourceLabel[entry.source] ?? entry.source}
                </span>
              </p>
              <p class="truncate text-[11px] text-muted-foreground">
                {entry.note} · {relative(entry.created_at)}
              </p>
            </div>
            <span
              class="shrink-0 text-sm font-semibold tabular-nums {entry.points < 0
                ? 'text-destructive'
                : 'text-primary'}"
            >
              {entry.points > 0 ? '+' : ''}{entry.points}
            </span>
            <button
              class="touch-target shrink-0 text-muted-foreground hover:text-destructive"
              onclick={() => revoke(entry)}
              aria-label="Eintrag zurücknehmen"
              title="Zurücknehmen"
            >
              <Undo2 class="h-4 w-4" />
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="card mb-4 p-5">
    <header class="mb-4 flex items-center justify-between">
      <h2 class="flex items-center gap-2 font-semibold">
        <MonitorSmartphone class="h-5 w-5" /> Geräte
      </h2>
      <button
        class="btn-primary px-3"
        onclick={() => (showDeviceForm ? (showDeviceForm = false) : startNewDevice())}
        aria-label="Gerät hinzufügen"
      >
        {#if showDeviceForm}<X class="h-5 w-5" />{:else}<Plus class="h-5 w-5" />{/if}
      </button>
    </header>

    {#if deviceError}
      <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {deviceError}
      </p>
    {/if}

    {#if showDeviceForm}
      <form class="mb-4 space-y-3 rounded-xl border border-border p-4" onsubmit={saveDevice}>
        <div class="grid gap-2 sm:grid-cols-2">
          <label class="text-sm">
            Name
            <input class="input mt-1" bind:value={deviceDraft.name} maxlength="40" required />
          </label>
          <label class="text-sm">
            Prüfung über
            <select class="input mt-1" bind:value={deviceDraft.type}>
              <option value="http">Webadresse (HTTP)</option>
              <option value="tcp">Port (TCP)</option>
            </select>
          </label>
        </div>

        {#if deviceDraft.type === 'http'}
          <label class="block text-sm">
            Adresse für die Prüfung
            <input
              class="input mt-1"
              bind:value={deviceDraft.url}
              placeholder="http://192.168.1.20:32400/identity"
              inputmode="url"
              autocapitalize="off"
              spellcheck="false"
            />
          </label>
          <label class="block text-sm">
            Erwarteter Status (0 = egal, solange geantwortet wird)
            <input class="input mt-1" type="number" min="0" max="599" bind:value={deviceDraft.expect_status} />
          </label>
        {:else}
          <div class="grid gap-2 sm:grid-cols-2">
            <label class="text-sm">
              Host
              <input class="input mt-1" bind:value={deviceDraft.host} placeholder="192.168.1.20" />
            </label>
            <label class="text-sm">
              Port
              <input class="input mt-1" type="number" min="1" max="65535" bind:value={deviceDraft.port} />
            </label>
          </div>
        {/if}

        <label class="block text-sm">
          Oberfläche zum Antippen (optional)
          <input
            class="input mt-1"
            bind:value={deviceDraft.link}
            placeholder="http://192.168.1.20:32400/web"
            inputmode="url"
            autocapitalize="off"
            spellcheck="false"
          />
        </label>

        {#if hostWarnung}
          <p class="flex items-start gap-2 rounded-lg bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-400">
            <TriangleAlert class="mt-0.5 h-4 w-4 shrink-0" />
            <span>
              Geprüft wird <strong>{hostWarnung.pruefung}</strong>, geöffnet wird
              <strong>{hostWarnung.ziel}</strong>. Dann leuchtet die Kachel grün
              und führt trotzdem woanders hin. Falls das Absicht ist, einfach
              speichern.
            </span>
          </p>
        {/if}

        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" class="h-4 w-4 rounded" bind:checked={deviceDraft.enabled} />
          Aktiv (wird geprüft und angezeigt)
        </label>

        {#if testResult}
          <p
            class="rounded-lg px-3 py-2 text-sm {testResult.status === 'up'
              ? 'bg-success/10 text-success'
              : 'bg-destructive/10 text-destructive'}"
          >
            {#if testResult.status === 'up'}
              ✓ Erreichbar in {testResult.latency_ms} ms
            {:else}
              ✗ Nicht erreichbar: {testResult.error}
            {/if}
          </p>
        {/if}

        <div class="flex gap-2">
          <button type="button" class="btn-outline flex-1 text-sm" onclick={testDevice} disabled={testing}>
            <Zap class="h-4 w-4" />
            {testing ? 'Teste…' : 'Verbindung testen'}
          </button>
          <button class="btn-primary flex-1 text-sm" disabled={busy || !deviceDraft.name.trim()}>
            {editingDevice ? 'Speichern' : 'Anlegen'}
          </button>
        </div>
      </form>
    {/if}

    {#if devices.length === 0}
      <p class="py-6 text-center text-sm text-muted-foreground">
        Noch keine Geräte. Mit <strong>+</strong> das erste anlegen.
      </p>
    {:else}
      <ul class="space-y-2">
        {#each devices as device, index (device.id)}
          <li
            class="flex items-center gap-2 rounded-xl bg-muted/30 p-3 {device.enabled
              ? ''
              : 'opacity-55'}"
          >
            <div class="flex shrink-0 flex-col">
              <button
                class="text-muted-foreground disabled:opacity-30"
                onclick={() => moveDevice(index, -1)}
                disabled={index === 0}
                aria-label="{device.name} nach oben"
              >
                <ChevronUp class="h-4 w-4" />
              </button>
              <button
                class="text-muted-foreground disabled:opacity-30"
                onclick={() => moveDevice(index, 1)}
                disabled={index === devices.length - 1}
                aria-label="{device.name} nach unten"
              >
                <ChevronDown class="h-4 w-4" />
              </button>
            </div>

            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{device.name}</p>
              <p class="truncate text-xs text-muted-foreground">
                {device.type === 'http' ? device.url : `${device.host}:${device.port}`}
                {#if !device.enabled}· inaktiv{/if}
              </p>
            </div>

            <button
              class="btn-ghost shrink-0 px-2 text-xs"
              onclick={() => toggleDevice(device)}
              title={device.enabled ? 'Deaktivieren' : 'Aktivieren'}
            >
              {device.enabled ? 'Aktiv' : 'Inaktiv'}
            </button>
            <button class="btn-ghost shrink-0 px-3 text-sm" onclick={() => startEditDevice(device)}>
              Bearbeiten
            </button>
            <button
              class="touch-target shrink-0 text-muted-foreground hover:text-destructive"
              onclick={() => removeDevice(device)}
              aria-label="{device.name} entfernen"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="card mb-4 p-5">
    <header class="mb-4 flex items-center justify-between">
      <h2 class="flex items-center gap-2 font-semibold">
        <Music class="h-5 w-5" /> Musik
      </h2>
      {#if musik?.enabled && musik.available}
        <button
          class="btn-ghost shrink-0 rounded-full px-2 text-muted-foreground"
          onclick={musikNeuEinlesen}
          disabled={musik.progress.running}
          title="Neu einlesen"
          aria-label="Musikordner neu einlesen"
        >
          <RefreshCw class="h-4 w-4 {musik.progress.running ? 'animate-spin' : ''}" />
        </button>
      {/if}
    </header>

    {#if musikFehler}
      <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {musikFehler}
      </p>
    {/if}

    {#if !musik}
      <p class="py-4 text-center text-sm text-muted-foreground">Lade…</p>
    {:else if !musik.enabled}
      <p class="text-sm text-muted-foreground">
        Es ist kein Ordner eingehängt. Dafür braucht es eine Zeile in der
        <code>.env</code> und einen Neustart mit <code>make up</code>:
      </p>
      <pre class="mt-2 overflow-x-auto rounded-lg bg-muted/40 px-3 py-2 text-xs"><code
          >MUSIC_HOST_DIR=/media/festplatte/AUDIO</code
        ></pre>
    {:else}
      <!--
        Der Unterschied, der hier erklärt werden muss: WELCHER Ordner des
        Rechners hereingereicht wird, steht in der .env — ein Container sieht
        nur, was in ihn eingehängt wurde, und daran ändert keine
        Weboberfläche etwas. WELCHER Teil davon gehört wird, steht hier.
      -->
      <p class="mb-3 text-sm text-muted-foreground">
        Eingehängt ist <code class="text-foreground">{musik.mount}</code>. Welcher
        Ordner des Rechners das ist, steht in der <code>.env</code> unter
        <code>MUSIC_HOST_DIR</code> — das lässt sich von hier aus nicht ändern.
        Welchen Teil davon ihr hört, schon.
      </p>

      <div class="mb-3 rounded-xl bg-muted/30 p-3 text-sm">
        <p>
          <span class="text-muted-foreground">Es gilt:</span>
          <strong>{musik.subdir || 'der ganze eingehängte Ordner'}</strong>
        </p>
        <p class="mt-0.5 text-xs text-muted-foreground">
          {#if musik.progress.running}
            Wird eingelesen… {musik.progress.scanned.toLocaleString('de-DE')} Dateien
          {:else if !musik.available}
            Gerade nicht erreichbar — ist die Festplatte angeschlossen?
          {:else}
            {musik.tracks.toLocaleString('de-DE')} Titel im Index
          {/if}
        </p>
      </div>

      {#if !musikOffen}
        <button
          class="btn-outline w-full text-sm"
          onclick={() => {
            musikOffen = true;
            void musikBlaettern(musik?.subdir ?? '');
          }}
        >
          <Folder class="h-4 w-4" /> Anderen Ordner wählen
        </button>
      {:else}
        <div class="rounded-xl border border-border p-3">
          <div class="mb-2 flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
            <button class="rounded px-1.5 py-0.5 hover:bg-accent" onclick={() => musikBlaettern('')}>
              Eingehängter Ordner
            </button>
            {#each musikPfadTeile as teil (teil.path)}
              <ChevronRight class="h-3 w-3 shrink-0 opacity-50" />
              <button
                class="max-w-[10rem] truncate rounded px-1.5 py-0.5 hover:bg-accent"
                onclick={() => musikBlaettern(teil.path)}
              >
                {teil.name}
              </button>
            {/each}
          </div>

          {#if musikOrdner}
            <button
              class="mb-2 w-full rounded-lg bg-primary/10 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-primary/15"
              onclick={() => musikOrdnerWaehlen(musikOrdner?.path ?? '')}
              disabled={busy}
            >
              Diesen Ordner verwenden
              {#if musikOrdner.has_audio}
                <span class="block text-xs opacity-80">Hier liegen Audiodateien</span>
              {:else}
                <span class="block text-xs opacity-80">
                  Hier liegen keine Audiodateien — die Unterordner zählen mit
                </span>
              {/if}
            </button>

            <ul class="scrollbar-thin max-h-64 space-y-1 overflow-y-auto pr-1">
              {#if musikOrdner.path}
                <li>
                  <button
                    class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent"
                    onclick={() => musikBlaettern(musikOrdner?.parent ?? '')}
                  >
                    <CornerLeftUp class="h-4 w-4 shrink-0" /> Eine Ebene höher
                  </button>
                </li>
              {/if}
              {#each musikOrdner.folders as eintrag (eintrag.path)}
                <li>
                  <button
                    class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left transition-colors hover:bg-accent"
                    onclick={() => musikBlaettern(eintrag.path)}
                    disabled={!eintrag.has_subfolders && !eintrag.has_audio}
                  >
                    <Folder class="h-4 w-4 shrink-0 text-muted-foreground" />
                    <span class="min-w-0 flex-1 truncate text-sm">{eintrag.name}</span>
                    {#if eintrag.has_audio}
                      <Music class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    {/if}
                  </button>
                </li>
              {/each}
              {#if musikOrdner.folders.length === 0}
                <li class="py-4 text-center text-xs text-muted-foreground">
                  Keine Unterordner.
                </li>
              {/if}
            </ul>
          {:else}
            <p class="py-4 text-center text-sm text-muted-foreground">Lade…</p>
          {/if}

          <button class="btn-ghost mt-2 w-full text-sm" onclick={() => (musikOffen = false)}>
            Schließen
          </button>
        </div>
      {/if}
    {/if}
  </section>

  <section class="card p-5">
    <header class="mb-4 flex items-center justify-between">
      <h2 class="flex items-center gap-2 font-semibold">
        <Database class="h-5 w-5" /> Backups
      </h2>
      <div class="flex gap-2">
        <a class="btn-outline px-3 text-sm" href={adminApi.downloadUrl()} download>
          <Download class="h-4 w-4" />
          <span class="hidden sm:inline">Herunterladen</span>
        </a>
        <button class="btn-primary px-3 text-sm" onclick={runBackup} disabled={busy}>
          <HardDriveDownload class="h-4 w-4" />
          <span class="hidden sm:inline">Jetzt sichern</span>
        </button>
      </div>
    </header>

    <p class="mb-3 text-xs text-muted-foreground">
      Automatisch jede Nacht um 3 Uhr, 7 Tage Aufbewahrung. Gespeichert unter
      <code>backend/data/backup/</code>.
    </p>

    {#if backups.length === 0}
      <p class="py-4 text-center text-sm text-muted-foreground">Noch keine Backups</p>
    {:else}
      <ul class="scrollbar-thin max-h-64 space-y-1 overflow-y-auto pr-1">
        {#each backups as file (file.name)}
          <li class="flex items-center justify-between gap-2 rounded bg-muted/30 px-3 py-2 text-xs">
            <span class="truncate font-mono">{file.name}</span>
            <span class="shrink-0 text-muted-foreground">{size(file.size)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="card mt-4 p-5">
    <h2 class="mb-2 flex items-center gap-2 font-semibold">
      <MonitorSmartphone class="h-5 w-5" /> Wandgerät
    </h2>
    <p class="mb-4 text-sm text-muted-foreground">
      Ein Tablet, das fest an der Wand hängt, wird hier zum Familien-Gerät.
      Es bleibt dauerhaft angemeldet, zeigt aber keine persönlichen Daten:
      keine Rangliste, keine Einstellungen, keine eigenen Links. Wer eine
      Aufgabe abhakt, wird kurz gefragt, wer er ist — ohne PIN.
      <br /><br />
      Beim Einrichten wirst du auf diesem Gerät <strong>abgemeldet</strong>,
      damit nicht versehentlich alles auf dein Konto läuft. Diese Einstellung
      gilt nur für <strong>dieses</strong> Gerät — dein Handy bleibt
      unberührt.
    </p>

    {#if geraetFehler}
      <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {geraetFehler}
      </p>
    {/if}

    {#if istWandgeraet}
      <div class="flex flex-wrap items-center gap-3">
        <span class="flex items-center gap-2 text-sm text-primary">
          <Check class="h-4 w-4" /> Dieses Gerät ist ein Wandgerät.
        </span>
        <button class="btn-outline px-3 text-sm" onclick={wandgeraetAus} disabled={busy}>
          Wieder aufheben
        </button>
      </div>
    {:else}
      <button class="btn-primary px-4 text-sm" onclick={wandgeraetAn} disabled={busy}>
        <MonitorSmartphone class="h-4 w-4" />
        Dieses Gerät als Wandgerät einrichten
      </button>
    {/if}
  </section>
</div>
