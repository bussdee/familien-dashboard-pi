<script lang="ts">
  import type { Snippet } from 'svelte';

  /** Ein Schalter mit Beschriftung und einem Satz darunter, der sagt, was er tut. */
  let {
    an,
    titel,
    hinweis = '',
    gesperrt = false,
    onUmschalten,
    children,
  }: {
    an: boolean;
    titel: string;
    hinweis?: string;
    gesperrt?: boolean;
    onUmschalten: (neu: boolean) => void;
    children?: Snippet;
  } = $props();
</script>

<div class="flex items-start gap-3 py-3">
  <div class="min-w-0 flex-1">
    <p class="text-sm font-medium">{titel}</p>
    {#if hinweis}<p class="mt-0.5 text-xs text-muted-foreground">{hinweis}</p>{/if}
    {@render children?.()}
  </div>
  <button
    type="button"
    role="switch"
    aria-checked={an}
    aria-label={titel}
    disabled={gesperrt}
    class="relative mt-0.5 h-7 w-12 shrink-0 rounded-full transition-colors disabled:opacity-50
      {an ? 'bg-primary' : 'bg-muted-foreground/30'}"
    onclick={() => onUmschalten(!an)}
  >
    <span
      class="absolute left-0.5 top-0.5 h-6 w-6 rounded-full bg-white shadow transition-transform
        {an ? 'translate-x-5' : ''}"
    ></span>
  </button>
</div>
