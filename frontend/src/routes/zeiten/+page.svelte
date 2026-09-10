<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Check, Clock, Copy, Plus, Trash2 } from 'lucide-svelte';
  import { ApiError, adminApi, timesApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import type { DayTime, TimeKind, TimePerson, User, WeeklyTime } from '$lib/types';

  /**
   * Zwei Reiter, weil zwei verschiedene Leben eingetragen werden.
   *
   * Das Kind hat einen Stundenplan: ein Wochenmuster, das ein- bis zweimal im
   * Jahr angefasst wird. Die Eltern arbeiten jede Woche anders und tragen
   * einmal im Monat die nächsten vier Wochen ein.
   *
   * Deshalb sieht der zweite Reiter aus wie ein Kalenderblatt und nicht wie
   * ein Formular: Vier Wochen am Stück, Uhrzeiten direkt in die Felder. Wer
   * für jeden Tag einzeln ein Formular öffnen müsste, trägt es kein zweites
   * Mal ein.
   */
  type Reiter = 'wochenplan' | 'kalender';
  let reiter = $state<Reiter>('kalender');

  const WOCHENTAGE = ['Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag', 'Sonntag'];
  const ARTEN: { wert: TimeKind; label: string }[] = [
    { wert: 'arbeit', label: 'Arbeit' },
    { wert: 'schule', label: 'Schule' },
    { wert: 'frei', label: 'Frei' },
    { wert: 'urlaub', label: 'Urlaub' },
    { wert: 'krank', label: 'Krank' },
    { wert: 'sonstiges', label: 'Sonstiges' },
  ];

  let personen = $state<TimePerson[]>([]);
  let muster = $state<WeeklyTime[]>([]);
  let fehler = $state('');
  let hinweis = $state('');
  let busy = $state(false);
  let loading = $state(true);

  const ich = $derived($session.user);
  const istAdmin = $derived($session.user?.role === 'admin');
  /** Für wen wird gerade eingetragen. Wer kein Admin ist, kann nur sich selbst. */
  let fuer = $state(0);
  const bearbeitbar = $derived(istAdmin ? personen : personen.filter((p) => p.id === ich?.id));

  // ───────────────────────── Wochenplan ─────────────────────────
  let neuerTag = $state(0);
  let neuVon = $state('08:00');
  let neuBis = $state('13:00');
  let neuArt = $state<TimeKind>('schule');
  let neuNotiz = $state('');

  const meinMuster = $derived(
    muster.filter((m) => m.user_id === fuer).sort((a, b) =>
      a.weekday === b.weekday ? a.start_time.localeCompare(b.start_time) : a.weekday - b.weekday,
    ),
  );

  async function musterAnlegen(event: SubmitEvent) {
    event.preventDefault();
    fehler = '';
    busy = true;
    try {
      await timesApi.createWeekly({
        user_id: fuer, weekday: neuerTag, start_time: neuVon,
        end_time: neuBis, kind: neuArt, note: neuNotiz.trim(),
      });
      muster = await timesApi.weekly();
      neuNotiz = '';
      hinweis = 'Eingetragen';
      setTimeout(() => (hinweis = ''), 3000);
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Speichern fehlgeschlagen';
    } finally {
      busy = false;
    }
  }

  async function musterEntfernen(m: WeeklyTime) {
    const ok = await confirmAction({
      title: `${WOCHENTAGE[m.weekday]} ${m.start_time}–${m.end_time} entfernen?`,
    });
    if (!ok) return;
    try {
      await timesApi.removeWeekly(m.id);
      muster = await timesApi.weekly();
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Löschen fehlgeschlagen';
    }
  }

  // ───────────────────────── Vier Wochen ─────────────────────────
  /** Der Montag der laufenden Woche — Kalenderblätter fangen montags an. */
  function montagDieserWoche(): string {
    const d = new Date();
    const versatz = (d.getDay() + 6) % 7;
    d.setDate(d.getDate() - versatz);
    return d.toISOString().slice(0, 10);
  }

  let start = $state(montagDieserWoche());
  const tage = $derived.by(() => {
    const liste: string[] = [];
    const d = new Date(start + 'T12:00:00');
    for (let i = 0; i < 28; i++) {
      liste.push(new Date(d.getTime() + i * 86400000).toISOString().slice(0, 10));
    }
    return liste;
  });

  /** Der Entwurf: Tag -> {von, bis, art}. Wird erst beim Speichern geschickt. */
  let entwurf = $state<Record<string, { von: string; bis: string; art: TimeKind }>>({});

  function leer() {
    return { von: '', bis: '', art: 'arbeit' as TimeKind };
  }

  async function tageLaden() {
    const bis = tage[tage.length - 1];
    try {
      const { entries } = await timesApi.days(start, bis);
      const neu: Record<string, { von: string; bis: string; art: TimeKind }> = {};
      for (const t of tage) neu[t] = leer();
      for (const e of entries as DayTime[]) {
        if (e.user_id !== fuer) continue;
        neu[e.day] = { von: e.start_time, bis: e.end_time, art: e.kind };
      }
      entwurf = neu;
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Zeiten konnten nicht geladen werden';
    }
  }

  /**
   * Die Woche darüber kopieren. Wer Schicht arbeitet, hat oft zwei ähnliche
   * Wochen hintereinander — dann ist das ein Tastendruck statt vierzehn.
   */
  function wocheKopieren(wocheNr: number) {
    if (wocheNr === 0) return;
    const neu = { ...entwurf };
    for (let i = 0; i < 7; i++) {
      const quelle = tage[(wocheNr - 1) * 7 + i];
      const ziel = tage[wocheNr * 7 + i];
      neu[ziel] = { ...(entwurf[quelle] ?? leer()) };
    }
    entwurf = neu;
  }

  async function speichern() {
    fehler = '';
    busy = true;
    try {
      const eintraege = tage.map((tag) => {
        const e = entwurf[tag] ?? leer();
        return {
          user_id: fuer, day: tag,
          start_time: e.von, end_time: e.bis, kind: e.art,
        };
      });
      const { saved, removed } = await timesApi.saveDays(eintraege);
      hinweis = `${saved} Tage gespeichert${removed > 0 ? `, ${removed} entfernt` : ''}`;
      setTimeout(() => (hinweis = ''), 4000);
      await tageLaden();
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Speichern fehlgeschlagen';
    } finally {
      busy = false;
    }
  }

  const tagKurz = (iso: string) =>
    new Date(iso + 'T12:00:00').toLocaleDateString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit' });

  const istWochenende = (iso: string) => {
    const t = new Date(iso + 'T12:00:00').getDay();
    return t === 0 || t === 6;
  };

  onMount(() => {
    void (async () => {
      // Am Wandgerät gehört diese Seite niemandem — dort führt sie nur in
      // eine Sperre.
      if ($session.ready && $session.device) {
        void goto('/');
        return;
      }
      try {
        const [uebersicht, wochen] = await Promise.all([
          timesApi.overview(undefined, 1),
          timesApi.weekly(),
        ]);
        personen = uebersicht.people;
        muster = wochen;
        fuer = ich?.id ?? personen[0]?.id ?? 0;
        await tageLaden();
      } catch (e) {
        fehler = e instanceof ApiError ? e.message : 'Laden fehlgeschlagen';
      } finally {
        loading = false;
      }
    })();
  });

  // Wechselt die Person, muss der Entwurf neu geholt werden — sonst stünden
  // dort die Zeiten des Vorherigen.
  let letzteFuer = 0;
  $effect(() => {
    if (fuer && fuer !== letzteFuer && !loading) {
      letzteFuer = fuer;
      void tageLaden();
    }
  });
</script>

<svelte:head><title>Zeiten · Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-4xl px-4 py-6">
  <h1 class="mb-2 flex items-center gap-2 text-2xl font-semibold">
    <Clock class="h-6 w-6" /> Arbeit & Schule
  </h1>
  <p class="mb-6 text-sm text-muted-foreground">
    Wer wann weg ist. Daraus rechnet die Übersicht aus, ab wann alle zu Hause
    sind.
  </p>

  {#if fehler}
    <p class="mb-4 rounded-lg bg-destructive/10 px-4 py-2 text-sm text-destructive">{fehler}</p>
  {/if}
  {#if hinweis}
    <p class="mb-4 flex items-center gap-2 rounded-lg bg-success/10 px-4 py-2 text-sm text-success">
      <Check class="h-4 w-4" />
      {hinweis}
    </p>
  {/if}

  {#if loading}
    <p class="py-10 text-center text-sm text-muted-foreground">Lade…</p>
  {:else}
    {#if bearbeitbar.length > 1}
      <label class="mb-4 block text-sm">
        Für wen
        <select class="input mt-1" bind:value={fuer}>
          {#each bearbeitbar as p (p.id)}
            <option value={p.id}>{p.avatar_emoji} {p.name}</option>
          {/each}
        </select>
      </label>
    {/if}

    <div class="mb-5 flex gap-1 rounded-xl bg-muted/40 p-1">
      <button
        class="flex-1 rounded-lg px-3 py-2 text-sm transition-colors {reiter === 'kalender'
          ? 'bg-card font-medium shadow-sm'
          : 'text-muted-foreground'}"
        onclick={() => (reiter = 'kalender')}
      >
        Nächste vier Wochen
      </button>
      <button
        class="flex-1 rounded-lg px-3 py-2 text-sm transition-colors {reiter === 'wochenplan'
          ? 'bg-card font-medium shadow-sm'
          : 'text-muted-foreground'}"
        onclick={() => (reiter = 'wochenplan')}
      >
        Fester Wochenplan
      </button>
    </div>

    {#if reiter === 'wochenplan'}
      <section class="card p-5">
        <p class="mb-4 text-sm text-muted-foreground">
          Für alles, was jede Woche gleich ist — ein Stundenplan zum Beispiel.
          Gilt, bis er geändert wird. <strong>Ein eingetragener Tag im
          Kalender sticht den Wochenplan</strong>, ein Feiertag hebt ihn also
          auf, ohne ihn zu löschen.
        </p>

        <form class="mb-5 grid gap-2 sm:grid-cols-[1fr_auto_auto_auto]" onsubmit={musterAnlegen}>
          <select class="input" bind:value={neuerTag} aria-label="Wochentag">
            {#each WOCHENTAGE as tag, i (tag)}
              <option value={i}>{tag}</option>
            {/each}
          </select>
          <input class="input sm:w-28" type="time" bind:value={neuVon} aria-label="Von" />
          <input class="input sm:w-28" type="time" bind:value={neuBis} aria-label="Bis" />
          <select class="input sm:w-36" bind:value={neuArt} aria-label="Art">
            {#each ARTEN.filter((a) => a.wert === 'schule' || a.wert === 'arbeit' || a.wert === 'sonstiges') as a (a.wert)}
              <option value={a.wert}>{a.label}</option>
            {/each}
          </select>
          <input
            class="input sm:col-span-3"
            placeholder="Notiz, z. B. „mit Bus 4“ (optional)"
            bind:value={neuNotiz}
            maxlength="60"
          />
          <button class="btn-primary sm:col-span-1" disabled={busy}>
            <Plus class="h-4 w-4" /> Eintragen
          </button>
        </form>

        {#if meinMuster.length === 0}
          <p class="py-6 text-center text-sm text-muted-foreground">
            Noch kein fester Wochenplan.
          </p>
        {:else}
          <ul class="space-y-1.5">
            {#each meinMuster as m (m.id)}
              <li class="flex items-center gap-3 rounded-lg bg-muted/30 px-3 py-2.5">
                <span class="w-24 shrink-0 text-sm font-medium">{WOCHENTAGE[m.weekday]}</span>
                <span class="shrink-0 text-sm tabular-nums">{m.start_time}–{m.end_time}</span>
                <span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                  {ARTEN.find((a) => a.wert === m.kind)?.label ?? m.kind}
                  {#if m.note}· {m.note}{/if}
                </span>
                <button
                  class="touch-target shrink-0 text-muted-foreground hover:text-destructive"
                  onclick={() => musterEntfernen(m)}
                  aria-label="Eintrag entfernen"
                >
                  <Trash2 class="h-4 w-4" />
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    {:else}
      <section class="card p-5">
        <p class="mb-4 text-sm text-muted-foreground">
          Vier Wochen am Stück. Leere Felder heissen „nichts Besonderes" —
          dann gilt der Wochenplan, falls es einen gibt. Gespeichert wird
          alles auf einmal.
        </p>

        {#each [0, 1, 2, 3] as woche (woche)}
          <div class="mb-4">
            <div class="mb-1.5 flex items-center justify-between">
              <h2 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Woche {woche + 1}
              </h2>
              {#if woche > 0}
                <button
                  class="btn-ghost px-2 text-xs text-muted-foreground"
                  onclick={() => wocheKopieren(woche)}
                  title="Die Woche darüber übernehmen"
                >
                  <Copy class="h-3.5 w-3.5" /> wie Woche {woche}
                </button>
              {/if}
            </div>

            <div class="space-y-1">
              {#each tage.slice(woche * 7, woche * 7 + 7) as tag (tag)}
                <div
                  class="grid grid-cols-[7rem_1fr_1fr_auto] items-center gap-2 rounded-lg px-2 py-1.5 {istWochenende(
                    tag,
                  )
                    ? 'bg-muted/20'
                    : ''}"
                >
                  <span class="truncate text-sm {istWochenende(tag) ? 'text-muted-foreground' : ''}">
                    {tagKurz(tag)}
                  </span>
                  <input
                    class="input py-1.5 text-sm"
                    type="time"
                    bind:value={entwurf[tag].von}
                    aria-label="Von am {tag}"
                  />
                  <input
                    class="input py-1.5 text-sm"
                    type="time"
                    bind:value={entwurf[tag].bis}
                    aria-label="Bis am {tag}"
                  />
                  <select
                    class="input w-28 py-1.5 text-sm"
                    bind:value={entwurf[tag].art}
                    aria-label="Art am {tag}"
                  >
                    {#each ARTEN as a (a.wert)}
                      <option value={a.wert}>{a.label}</option>
                    {/each}
                  </select>
                </div>
              {/each}
            </div>
          </div>
        {/each}

        <button class="btn-primary w-full" onclick={speichern} disabled={busy}>
          <Check class="h-4 w-4" />
          {busy ? 'Speichere…' : 'Vier Wochen speichern'}
        </button>
      </section>
    {/if}
  {/if}
</div>
