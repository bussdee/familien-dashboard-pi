<script lang="ts">
  import { untrack } from 'svelte';
  import { Minus, Plus, Sparkles } from 'lucide-svelte';
  import { ApiError, adminApi } from '$lib/api';
  import { board, mitVorzeichen } from '$lib/stores/scores.svelte';
  import { punkte } from '$lib/stores/punkte.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from './Modal.svelte';

  /**
   * Punkte von Hand vergeben oder abziehen.
   *
   * Bis 2.0 hiess das: in der Verwaltung eine Zahl eintippen, zum Abziehen
   * eine negative. Auf vielen Handy-Tastaturen gibt es im Zahlenfeld aber gar
   * kein Minus — wer abziehen wollte, kam schlicht nicht weiter. Jetzt sagen
   * zwei Knöpfe, was passieren soll, und die Zahl ist immer positiv.
   *
   * Dazu: Gesichter statt Auswahlliste, mehrere auf einmal („alle Kinder
   * +10"), Beträge und Gründe zum Antippen, und ein Rückgängig danach.
   */
  const BETRAEGE = [5, 10, 20, 50];
  const GRUENDE = {
    plus: ['Toll geholfen', 'Ohne Aufforderung', 'Hausaufgaben', 'Lieb zu den Geschwistern', 'Zimmer aufgeräumt'],
    minus: ['Aufgabe vergessen', 'Streit', 'Nicht aufgeräumt', 'Verklickt'],
  };

  let auswahl = $state<number[]>([]);
  let betrag = $state(10);
  /** Das freie Feld als Text: so gibt es keine Tastatur ohne Ziffern und kein „e". */
  let eigener = $state('');
  let grund = $state('');
  let arbeitet = $state(false);
  let fehler = $state('');

  // Beim Öffnen die Vorauswahl übernehmen und alles andere zurücksetzen.
  // Nur das Öffnen zählt: Lädt die Rangliste danach nach, bleibt die Auswahl.
  $effect(() => {
    if (!punkte.offen) return;
    untrack(() => {
      auswahl = [...punkte.vorauswahl];
      betrag = 10;
      eigener = '';
      grund = '';
      fehler = '';
      if (!board.loaded) void board.refresh().catch(() => {});
    });
  });

  const personen = $derived([...board.scores].sort((a, b) => a.id - b.id));
  const plus = $derived(punkte.modus === 'plus');
  const menge = $derived.by(() => {
    if (eigener.trim() === '') return betrag;
    const n = Number.parseInt(eigener.replace(/\D/g, ''), 10);
    return Number.isFinite(n) ? n : 0;
  });
  const wert = $derived(plus ? menge : -menge);
  const gewaehlt = $derived(personen.filter((p) => auswahl.includes(p.id)));
  const alleGewaehlt = $derived(personen.length > 0 && auswahl.length === personen.length);
  const bereit = $derived(gewaehlt.length > 0 && menge > 0 && menge <= 10000 && !arbeitet);

  const namen = $derived.by(() => {
    const n = gewaehlt.map((p) => p.name);
    if (n.length <= 2) return n.join(' und ');
    return `${n.slice(0, -1).join(', ')} und ${n[n.length - 1]}`;
  });
  const knopf = $derived(
    gewaehlt.length === 0
      ? 'Wer bekommt die Punkte?'
      : `${mitVorzeichen(wert)} ${plus ? 'für' : 'bei'} ${namen}`,
  );

  function umschalten(id: number) {
    auswahl = auswahl.includes(id) ? auswahl.filter((x) => x !== id) : [...auswahl, id];
  }

  function alle() {
    auswahl = alleGewaehlt ? [] : personen.map((p) => p.id);
  }

  async function buchen(event: SubmitEvent) {
    event.preventDefault();
    if (!bereit) return;
    arbeitet = true;
    fehler = '';
    const text = grund.trim();
    const wer = namen;
    try {
      const res = await adminApi.adjustPoints(
        gewaehlt.map((p) => p.id),
        wert,
        text,
      );
      punkte.offen = false;
      punkte.gebucht++;
      await board.refresh().catch(() => {});
      toast(`${mitVorzeichen(wert)} ${plus ? 'für' : 'bei'} ${wer}${text ? ` · ${text}` : ''}`, {
        ton: plus ? 'erfolg' : 'info',
        aktion: {
          label: 'Rückgängig',
          run: async () => {
            await Promise.all(res.ids.map((id) => adminApi.revokePoints(id).catch(() => {})));
            punkte.gebucht++;
            await board.refresh().catch(() => {});
            toast('Buchung zurückgenommen');
          },
        },
      });
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Buchung fehlgeschlagen';
    } finally {
      arbeitet = false;
    }
  }
</script>

<Modal bind:open={punkte.offen} title="Punkte vergeben">
  <form class="space-y-5" onsubmit={buchen}>
    <!-- Erst die Richtung: zwei grosse Knöpfe statt eines Minuszeichens. -->
    <div class="grid grid-cols-2 gap-2 rounded-2xl bg-muted/50 p-1" role="radiogroup" aria-label="Gutschreiben oder abziehen">
      <button
        type="button"
        role="radio"
        aria-checked={plus}
        class="flex min-h-[48px] items-center justify-center gap-2 rounded-xl text-sm font-semibold transition-colors
          {plus ? 'bg-card text-success shadow-sm' : 'text-muted-foreground'}"
        onclick={() => (punkte.modus = 'plus')}
      >
        <Plus class="h-5 w-5" /> Gutschreiben
      </button>
      <button
        type="button"
        role="radio"
        aria-checked={!plus}
        class="flex min-h-[48px] items-center justify-center gap-2 rounded-xl text-sm font-semibold transition-colors
          {!plus ? 'bg-card text-destructive shadow-sm' : 'text-muted-foreground'}"
        onclick={() => (punkte.modus = 'minus')}
      >
        <Minus class="h-5 w-5" /> Abziehen
      </button>
    </div>

    <div>
      <div class="mb-2 flex items-center justify-between">
        <p class="text-sm font-medium">Für wen?</p>
        {#if personen.length > 1}
          <button type="button" class="text-xs font-medium text-primary" onclick={alle}>
            {alleGewaehlt ? 'Keinen' : 'Alle'}
          </button>
        {/if}
      </div>
      <div class="grid grid-cols-3 gap-2 sm:grid-cols-4">
        {#each personen as p (p.id)}
          {@const an = auswahl.includes(p.id)}
          <button
            type="button"
            class="flex flex-col items-center gap-1 rounded-2xl border p-2.5 transition-colors
              {an ? 'border-primary bg-primary/10' : 'border-[color:var(--haarlinie-stark)] hover:bg-accent'}"
            onclick={() => umschalten(p.id)}
            aria-pressed={an}
          >
            <span
              class="flex h-11 w-11 items-center justify-center rounded-full text-2xl"
              style="background-color: {p.color}22"
            >
              {p.avatar_emoji}
            </span>
            <span class="max-w-full truncate text-xs font-medium">{p.name}</span>
            <span class="text-[10px] tabular-nums text-muted-foreground">{p.total_points} P</span>
          </button>
        {/each}
      </div>
    </div>

    <div>
      <p class="mb-2 text-sm font-medium">Wie viele?</p>
      <div class="flex flex-wrap items-center gap-1.5">
        {#each BETRAEGE as b (b)}
          <button
            type="button"
            class="chip min-w-[3.25rem] justify-center font-semibold tabular-nums"
            aria-pressed={eigener.trim() === '' && betrag === b}
            onclick={() => {
              betrag = b;
              eigener = '';
            }}
          >
            {b}
          </button>
        {/each}
        <input
          class="input !w-24 !py-2 text-center tabular-nums"
          inputmode="numeric"
          pattern="[0-9]*"
          maxlength="5"
          placeholder="Andere"
          aria-label="Andere Anzahl"
          bind:value={eigener}
        />
      </div>
    </div>

    <div>
      <p class="mb-2 text-sm font-medium">Wofür? <span class="font-normal text-muted-foreground">(steht im Verlauf)</span></p>
      <div class="mb-2 flex flex-wrap gap-1.5">
        {#each GRUENDE[punkte.modus] as g (g)}
          <button type="button" class="chip" aria-pressed={grund === g} onclick={() => (grund = grund === g ? '' : g)}>
            {g}
          </button>
        {/each}
      </div>
      <input class="input" placeholder="Oder eigener Grund" bind:value={grund} maxlength="80" />
    </div>

    {#if fehler}
      <p class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{fehler}</p>
    {/if}

    <button
      class="btn w-full min-h-[52px] text-base font-semibold
        {plus ? 'bg-success text-success-foreground hover:bg-success/90' : 'bg-destructive text-destructive-foreground hover:bg-destructive/90'}"
      disabled={!bereit}
    >
      {#if plus}<Sparkles class="h-5 w-5" />{:else}<Minus class="h-5 w-5" />{/if}
      <span class="truncate">{arbeitet ? 'Wird gebucht…' : knopf}</span>
    </button>
    {#if !plus}
      <p class="-mt-3 text-center text-[11px] text-muted-foreground">
        Ein Abzug senkt Punkte und Guthaben. Verklickt? Danach unten „Rückgängig".
      </p>
    {/if}
  </form>
</Modal>
