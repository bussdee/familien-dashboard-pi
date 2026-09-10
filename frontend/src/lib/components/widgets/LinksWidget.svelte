<script lang="ts">
  import { session } from '$lib/stores';
  import { onMount } from 'svelte';
  import { ExternalLink, Link as LinkIcon, Plus, Users } from 'lucide-svelte';
  import { linksApi } from '$lib/api';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
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

{#snippet zeile()}
  {#if $session.device}
    {links.length === 0 ? 'Keine geteilten Links' : `${links.length} für die Familie`}
  {:else}
    {links.length === 0 ? 'Nichts angepinnt' : `${links.length} angepinnt`}
  {/if}
{/snippet}

<!-- Die Verwaltungsseite gehört einer Person — am Wandgerät führt sie nur in
     eine Sperre. Deshalb dort kein Knopf. -->
{#snippet aktionen()}
  {#if !$session.device}
    <a class="btn-ghost px-3 text-sm text-muted-foreground" href="/links">Alle</a>
  {/if}
{/snippet}

<Kachel titel="Links" icon={LinkIcon} {zeile} {aktionen}>
  {#if loading}
    <div class="space-y-2">
      {#each Array(3) as _, i (i)}
        <div class="h-12 animate-pulse rounded-xl bg-muted/40"></div>
      {/each}
    </div>
  {:else if links.length === 0}
    {#if $session.device}
      <KachelLeer
        icon={LinkIcon}
        titel="Noch keine geteilten Links"
        hinweis="Wer sich anmeldet, kann einen Link mit der Familie teilen."
      />
    {:else}
      <KachelLeer
        icon={LinkIcon}
        titel="Nichts angepinnt"
        hinweis="Was du oft brauchst, direkt auf der Übersicht."
      >
        {#snippet aktion()}
          <a href="/links" class="btn-outline text-sm">
            <Plus class="h-4 w-4" /> Links anpinnen
          </a>
        {/snippet}
      </KachelLeer>
    {/if}
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
</Kachel>
