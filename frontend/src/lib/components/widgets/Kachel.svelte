<script lang="ts">
  import { X } from 'lucide-svelte';
  // ComponentType statt Component: Die Symbole von lucide-svelte sind noch
  // Komponenten der vierten Fassung. Der neue Component-Typ passt nicht auf
  // sie, und eine Umstellung der Bibliothek gehört nicht in diese Änderung.
  import type { ComponentType, Snippet } from 'svelte';

  /**
   * Der gemeinsame Rahmen aller Kacheln.
   *
   * Vorher hatte jede ihren eigenen: mal mit Symbol im Titel, mal mit Symbol
   * rechts, mal ganz ohne; mal mit Innenabstand, mal ohne; Meldungen und leere
   * Zustände jedes Mal anders gebaut. Auf einer Übersicht, die aus zehn
   * Kacheln nebeneinander besteht, fällt das auf.
   *
   * Was hier festgelegt ist: Symbol und Titel links, eine Zeile darunter, die
   * sagt wie viel oder was los ist, Knöpfe rechts. Darunter Meldungen, dann
   * der Inhalt. Was jede Kachel INNEN zeigt, bleibt ihre Sache.
   */
  let {
    titel,
    // Grossgeschrieben entgegengenommen, damit es unten direkt als
    // <Icon /> stehen kann.
    icon: Icon,
    zeile,
    aktionen,
    hinweis = '',
    fehler = '',
    onFehlerZu,
    onHinweisZu,
    /**
     * Kacheln, deren Inhalt bis an den Rand reichen soll — der Foto-Rahmen
     * etwa. Der Innenabstand wandert dann in die Kopfzeile, statt am Bild zu
     * kleben.
     */
    randlos = false,
    children,
  }: {
    titel: string;
    icon?: ComponentType;
    zeile?: Snippet;
    aktionen?: Snippet;
    hinweis?: string;
    fehler?: string;
    onFehlerZu?: () => void;
    onHinweisZu?: () => void;
    randlos?: boolean;
    children: Snippet;
  } = $props();
</script>

<section class="flaeche {randlos ? 'overflow-hidden' : ''}">
  <!--
    items-start statt items-center: Wird die Zeile darunter zweizeilig, soll
    der Knopf oben bleiben und nicht mit nach unten rutschen.
  -->
  <header
    class="flex items-start justify-between gap-2 {randlos ? 'p-5 pb-3' : 'mb-4'}"
  >
    <div class="min-w-0">
      <h2 class="widget-title">
        {#if Icon}
          <Icon class="h-5 w-5 shrink-0" />
        {/if}
        <span class="truncate">{titel}</span>
      </h2>
      {#if zeile}
        <p class="truncate text-sm text-muted-foreground">{@render zeile()}</p>
      {/if}
    </div>

    {#if aktionen}
      <div class="flex shrink-0 items-center gap-1">
        {@render aktionen()}
      </div>
    {/if}
  </header>

  {#if hinweis || fehler}
    <div class="{randlos ? 'px-5' : ''} pb-2 space-y-1">
      {#if hinweis}
        <p class="flex items-start justify-between gap-2 rounded-lg bg-success/10 px-3 py-2 text-sm text-success">
          <span class="min-w-0 flex-1">{hinweis}</span>
          {#if onHinweisZu}
            <button onclick={onHinweisZu} aria-label="Schließen">
              <X class="h-4 w-4 shrink-0" />
            </button>
          {/if}
        </p>
      {/if}
      {#if fehler}
        <p class="flex items-start justify-between gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <span class="min-w-0 flex-1">{fehler}</span>
          {#if onFehlerZu}
            <button onclick={onFehlerZu} aria-label="Schließen">
              <X class="h-4 w-4 shrink-0" />
            </button>
          {/if}
        </p>
      {/if}
    </div>
  {/if}

  {@render children()}
</section>
