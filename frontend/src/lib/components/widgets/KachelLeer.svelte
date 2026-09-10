<script lang="ts">
  import type { ComponentType, Snippet } from 'svelte';

  /**
   * Der leere Zustand einer Kachel.
   *
   * Auch der war überall anders: mal ein blasses Symbol mit zwei Zeilen, mal
   * nur ein Satz in Grau, mal gar nichts. Dabei ist es die Ansicht, die ein
   * neuer Benutzer als Erstes sieht — sie sollte sagen, was hier hingehört
   * und wie es dorthin kommt.
   */
  let {
    icon: Icon,
    titel,
    hinweis = '',
    aktion,
  }: {
    icon: ComponentType;
    titel: string;
    hinweis?: string;
    aktion?: Snippet;
  } = $props();
</script>

<!--
  flex-1: Ein leerer Zustand setzt sich in die Mitte dessen, was das Fenster
  an Höhe hat. Klebte er oben, sähe ein Fenster mit wenig Inhalt neben einem
  vollen aus wie abgeschnitten.
-->
<div class="flex flex-1 flex-col items-center justify-center gap-2 px-4 py-10 text-center">
  <Icon class="h-10 w-10 text-muted-foreground opacity-40" />
  <p class="text-sm font-medium">{titel}</p>
  {#if hinweis}
    <p class="max-w-[26rem] text-xs text-muted-foreground">{hinweis}</p>
  {/if}
  {#if aktion}
    <div class="mt-2">{@render aktion()}</div>
  {/if}
</div>
