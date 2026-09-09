<script lang="ts">
  import { onMount } from 'svelte';
  import { ExternalLink, Link as LinkIcon, Plus, Users } from 'lucide-svelte';
  import { linksApi } from '$lib/api';
  import type { Link } from '$lib/types';

  let links = $state<Link[]>([]);
  let loading = $state(true);

  const host = (url: string) => {
    try {
      return new URL(url).host.replace(/^www\./, '');
    } catch {
      return url;
    }
  };

  onMount(async () => {
    try {
      links = (await linksApi.pinned()).links;
    } catch {
      links = [];
    } finally {
      loading = false;
    }
  });
</script>

<section class="flaeche">
  <header class="mb-4 flex items-center justify-between">
    <div>
      <h2 class="widget-title"><LinkIcon class="h-5 w-5 shrink-0" /> Links</h2>
      <p class="text-sm text-muted-foreground">
        {links.length === 0 ? 'Nichts angepinnt' : `${links.length} angepinnt`}
      </p>
    </div>
    <a class="btn-ghost px-3 text-sm text-muted-foreground" href="/links">Alle</a>
  </header>

  {#if loading}
    <div class="space-y-2">
      {#each Array(3) as _, i (i)}
        <div class="h-12 animate-pulse rounded-xl bg-muted/40"></div>
      {/each}
    </div>
  {:else if links.length === 0}
    <a
      href="/links"
      class="flex flex-col items-center gap-2 rounded-xl py-8 text-center text-muted-foreground transition-colors hover:bg-accent/40"
    >
      <Plus class="h-8 w-8 opacity-40" />
      <p class="text-sm font-medium">Links anpinnen</p>
      <p class="text-xs">Was du oft brauchst, direkt auf der Übersicht</p>
    </a>
  {:else}
    <!-- Two columns from the smallest phone up: bookmarks are short. -->
    <div class="grid grid-cols-2 gap-2">
      {#each links as link (link.id)}
        <a
          href={link.url}
          target="_blank"
          rel="noopener noreferrer"
          class="group flex items-center gap-2.5 rounded-xl border border-[color:var(--haarlinie)] p-2.5 transition-colors hover:bg-muted/25"
        >
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-card text-lg">
            {link.emoji}
          </span>
          <span class="min-w-0 flex-1">
            <span class="flex items-center gap-1 truncate text-sm font-medium">
              {link.title}
              {#if link.shared}<Users class="h-3 w-3 shrink-0 text-muted-foreground" />{/if}
            </span>
            <span class="block truncate text-[11px] text-muted-foreground">{host(link.url)}</span>
          </span>
          <ExternalLink
            class="h-3.5 w-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100"
          />
        </a>
      {/each}
    </div>
  {/if}
</section>
