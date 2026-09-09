<script lang="ts">
  import {
    ListMusic, Pause, Play, Repeat, Repeat1, RotateCcw, RotateCw, Shuffle,
    SkipBack, SkipForward, Volume2, X,
  } from 'lucide-svelte';
  import { player, titelVon, zeit } from '$lib/stores/player.svelte';

  /**
   * Die Leiste am unteren Rand. Sie ist auf jeder Seite sichtbar, sobald etwas
   * läuft — deshalb steht sie im Seitenlayout und nicht in der Musik-Kachel.
   *
   * `gedimmt` ist für die Diashow: Dort verschwinden alle Bedienelemente,
   * solange niemand das Bild berührt.
   */
  let {
    gedimmt = false,
    /** Die Diashow legt sich mit z-[80] über alles — dort muss die Leiste höher. */
    ueberDiashow = false,
  }: { gedimmt?: boolean; ueberDiashow?: boolean } = $props();

  let listeOffen = $state(false);

  const track = $derived(player.current);
  const fortschritt = $derived(
    player.laenge > 0 ? Math.min(100, (player.position / player.laenge) * 100) : 0,
  );

  function suchen(event: Event) {
    const wert = Number((event.currentTarget as HTMLInputElement).value);
    if (player.laenge > 0) player.springe((wert / 100) * player.laenge);
  }
</script>

{#if player.aktiv && track}
  <div
    class="fixed inset-x-0 bottom-0 border-t border-border bg-card/95 backdrop-blur transition-opacity duration-500
      {ueberDiashow ? 'z-[90]' : 'z-40'}
      {gedimmt ? 'pointer-events-none opacity-0' : 'opacity-100'}"
  >
    {#if player.fehler}
      <p
        class="flex items-center justify-between gap-2 bg-destructive/10 px-4 py-1.5 text-xs text-destructive"
      >
        <span class="min-w-0 flex-1 truncate">{player.fehler}</span>
        <button onclick={() => (player.fehler = '')} aria-label="Schließen">
          <X class="h-3.5 w-3.5 shrink-0" />
        </button>
      </p>
    {:else if player.brauchtTipp}
      <!--
        Browser verbieten Ton, der nicht auf eine Berührung folgt. Nach einem
        Neustart des Wandtablets muss jemand einmal tippen. Dagegen lässt sich
        nichts machen — aber man kann es sagen, statt stumm zu bleiben.
      -->
      <p class="bg-amber-500/10 px-4 py-1.5 text-xs text-amber-700 dark:text-amber-400">
        Der Browser lässt Ton erst nach einer Berührung zu. Einmal auf ▶ tippen.
      </p>
    {/if}

    <!-- Die Titelliste der laufenden Warteschlange, ausklappbar. -->
    {#if listeOffen}
      <ul class="scrollbar-thin max-h-56 overflow-y-auto border-b border-border px-2 py-1.5">
        {#each player.queue as eintrag, i (eintrag.id)}
          <li>
            <button
              class="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left text-sm transition-colors
                {i === player.index ? 'bg-primary/10 font-medium text-primary' : 'hover:bg-accent'}"
              onclick={() => player.waehle(i)}
            >
              <span class="w-6 shrink-0 text-right text-xs tabular-nums text-muted-foreground">
                {i + 1}
              </span>
              <span class="min-w-0 flex-1 truncate">{titelVon(eintrag)}</span>
              {#if eintrag.duration > 0}
                <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
                  {zeit(eintrag.duration)}
                </span>
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}

    <!-- Der Fortschrittsbalken sitzt ganz oben und ist zugleich der Schieber. -->
    <div class="relative h-1 bg-muted">
      <div class="h-full bg-primary transition-[width]" style="width: {fortschritt}%"></div>
      <input
        class="absolute inset-0 h-full w-full cursor-pointer opacity-0"
        type="range"
        min="0"
        max="100"
        step="0.1"
        value={fortschritt}
        oninput={suchen}
        disabled={player.laenge === 0}
        aria-label="Position im Titel"
      />
    </div>

    <div class="safe-bottom flex items-center gap-2 px-3 py-2 sm:gap-3 sm:px-4">
      <div class="min-w-0 flex-1">
        <p class="truncate text-sm font-medium">{titelVon(track)}</p>
        <p class="truncate text-xs text-muted-foreground">
          {#if track.artist}{track.artist} · {/if}
          {zeit(player.position)}{#if player.laenge > 0}&nbsp;/&nbsp;{zeit(player.laenge)}{/if}
          {#if player.queue.length > 1}
            · {player.index + 1} von {player.queue.length}
          {/if}
        </p>
      </div>

      <!-- 15 Sekunden zurück: bei einem Hörbuch der meistgenutzte Knopf. -->
      <button
        class="btn-ghost hidden shrink-0 rounded-full px-2 text-muted-foreground sm:inline-flex"
        onclick={() => player.spule(-15)}
        aria-label="15 Sekunden zurück"
        title="15 Sekunden zurück"
      >
        <RotateCcw class="h-4 w-4" />
      </button>

      <button
        class="btn-ghost shrink-0 rounded-full px-2"
        onclick={() => player.zurueck()}
        aria-label="Voriger Titel"
      >
        <SkipBack class="h-5 w-5" />
      </button>

      <button
        class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground transition-transform active:scale-95"
        onclick={() => player.toggle()}
        aria-label={player.playing ? 'Pause' : 'Abspielen'}
      >
        {#if player.playing}
          <Pause class="h-5 w-5" />
        {:else}
          <Play class="h-5 w-5 translate-x-[1px]" />
        {/if}
      </button>

      <button
        class="btn-ghost shrink-0 rounded-full px-2"
        onclick={() => player.weiter()}
        aria-label="Nächster Titel"
      >
        <SkipForward class="h-5 w-5" />
      </button>

      <button
        class="btn-ghost hidden shrink-0 rounded-full px-2 text-muted-foreground sm:inline-flex"
        onclick={() => player.spule(30)}
        aria-label="30 Sekunden vor"
        title="30 Sekunden vor"
      >
        <RotateCw class="h-4 w-4" />
      </button>

      <button
        class="btn-ghost hidden shrink-0 rounded-full px-2 md:inline-flex
          {player.zufall ? 'text-primary' : 'text-muted-foreground'}"
        onclick={() => player.zufallUmschalten()}
        aria-label="Zufallswiedergabe"
        aria-pressed={player.zufall}
        title="Zufallswiedergabe"
      >
        <Shuffle class="h-4 w-4" />
      </button>

      <button
        class="btn-ghost hidden shrink-0 rounded-full px-2 md:inline-flex
          {player.wiederholen === 'aus' ? 'text-muted-foreground' : 'text-primary'}"
        onclick={() => player.wiederholungUmschalten()}
        aria-label="Wiederholen: {player.wiederholen}"
        title={player.wiederholen === 'titel'
          ? 'Diesen Titel wiederholen'
          : player.wiederholen === 'liste'
            ? 'Liste wiederholen'
            : 'Nicht wiederholen'}
      >
        {#if player.wiederholen === 'titel'}
          <Repeat1 class="h-4 w-4" />
        {:else}
          <Repeat class="h-4 w-4" />
        {/if}
      </button>

      <label class="hidden shrink-0 items-center gap-1.5 lg:flex">
        <Volume2 class="h-4 w-4 text-muted-foreground" />
        <input
          class="w-20 cursor-pointer accent-[color:var(--primary)]"
          type="range"
          min="0"
          max="1"
          step="0.05"
          value={player.volume}
          oninput={(e) => player.setVolume(Number((e.currentTarget as HTMLInputElement).value))}
          aria-label="Lautstärke"
        />
      </label>

      {#if player.queue.length > 1}
        <button
          class="btn-ghost shrink-0 rounded-full px-2 {listeOffen
            ? 'text-primary'
            : 'text-muted-foreground'}"
          onclick={() => (listeOffen = !listeOffen)}
          aria-label="Titelliste"
          aria-expanded={listeOffen}
        >
          <ListMusic class="h-4 w-4" />
        </button>
      {/if}

      <button
        class="btn-ghost shrink-0 rounded-full px-2 text-muted-foreground hover:text-destructive"
        onclick={() => player.stop()}
        aria-label="Wiedergabe beenden"
        title="Wiedergabe beenden"
      >
        <X class="h-4 w-4" />
      </button>
    </div>
  </div>
{/if}
