<script lang="ts">
  import { t } from '$lib/i18n';
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
    {links.length === 0 ? t('Keine geteilten Links') : t('{0} für die Familie', [links.length])}
  {:else}
    {links.length === 0 ? t('Nichts angepinnt') : `${links.length} angepinnt`}
  {/if}
{/snippet}

<!-- Die Verwaltungsseite gehört einer Person — am Wandgerät führt sie nur in
     eine Sperre. Deshalb dort kein Knopf. -->
{#snippet aktionen()}
  {#if !$session.device}
    <a class="btn-ghost px-3 text-sm text-muted-foreground" href="/links">{t('Alle')}</a>
  {/if}
{/snippet}

<Kachel ton="var(--ton-links)" titel={t('Links')} icon={LinkIcon} {zeile} {aktionen}>
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
        titel={t('Noch keine geteilten Links')}
        hinweis={t('Ein Elternteil kann einen Link mit der Familie teilen.')}
      />
    {:else}
      <KachelLeer
        icon={LinkIcon}
        titel={t('Nichts angepinnt')}
        hinweis={$session.user?.role === 'admin'
          ? t('Was du oft brauchst, direkt auf der Übersicht.')
          : t('Die Eltern pinnen hier an, was die Familie oft braucht.')}
      >
        {#snippet aktion()}
          {#if $session.user?.role === 'admin'}
            <a href="/links" class="btn-outline text-sm">
              <Plus class="h-4 w-4" /> {t('Links anpinnen')}
            </a>
          {/if}
        {/snippet}
      </KachelLeer>
    {/if}
  {:else}
    <!--
      Eine Zeile pro Link: Symbol, Name, rechts die Adresse. In zwei Spalten
      mit je zwei Zeilen blieb von einem Namen auf dem Handy die Hälfte
      übrig — und lesen soll man die Liste mit einem Blick.
    -->
    <ul class="divide-y divide-[color:var(--haarlinie)]">
      {#each links as link (link.id)}
        <li>
          <a
            href={link.url}
            target="_blank"
            rel="noopener noreferrer"
            title="{link.title} — {link.url}"
            class="group flex min-h-[2.75rem] items-center gap-3 rounded-lg px-1 py-1.5 transition-colors hover:bg-muted/25"
          >
            <span class="w-7 shrink-0 text-center text-xl leading-none">{link.emoji}</span>
            <!-- Der Name behält seine Breite; die Adresse nimmt, was übrig ist,
                 und wird als Erste gekürzt. -->
            <span class="min-w-0 truncate text-sm font-medium">{link.title}</span>
            {#if link.shared}<Users class="h-3 w-3 shrink-0 text-muted-foreground" />{/if}
            <span class="min-w-0 flex-1 truncate text-right text-xs text-muted-foreground">{host(link.url)}</span>
            <ExternalLink
              class="h-3.5 w-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100"
            />
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</Kachel>
