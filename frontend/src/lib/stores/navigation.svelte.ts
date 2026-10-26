import { t } from '$lib/i18n';
import {
  BookOpen, Clock, CloudSun, Gift, House, Images, LayoutGrid, Link as LinkIcon, Newspaper,
  Settings, Shield, ShoppingCart, Trophy, UtensilsCrossed,
} from 'lucide-svelte';
import type { ComponentType } from 'svelte';
import { einstellungen } from '$lib/stores/einstellungen.svelte';

/**
 * Alle Ziele der App an einer Stelle. Kopfleiste, Menü und die Leiste am
 * unteren Rand lesen von hier — vorher stand die Liste zweimal im Code und
 * lief bei jedem neuen Punkt auseinander.
 *
 * Das Menü ist nach Gruppen geordnet, nicht nach der Reihenfolge, in der die
 * Seiten entstanden sind: Was man täglich braucht (Alltag), was die Familie
 * miteinander teilt (Familie), was einem selbst gehört (Ich), was Eltern
 * einrichten (Verwaltung).
 */
export type NavGruppe = 'alltag' | 'familie' | 'ich' | 'verwaltung';

export interface NavZiel {
  href: string;
  label: string;
  /** Kurzform für die schmale Leiste unten. */
  kurz: string;
  /** Ein halber Satz unter dem Namen im Menü. */
  hinweis: string;
  icon: ComponentType;
  gruppe: NavGruppe;
  /** Steht in der Kopfleiste am grossen Bildschirm. */
  leiste: boolean;
  /** Nur mit persönlicher Anmeldung sinnvoll — am Wandgerät ausgeblendet. */
  persoenlich?: boolean;
  nurAdmin?: boolean;
  /** Gibt es nur, wenn die Verwaltung den Bereich eingeschaltet hat. */
  modul?: 'rezepte' | 'neuigkeiten';
}

export const GRUPPEN: { id: NavGruppe; label: string }[] = [
  { id: 'alltag', label: t('Alltag') },
  { id: 'familie', label: t('Familie') },
  { id: 'ich', label: t('Ich') },
  { id: 'verwaltung', label: t('Verwaltung') },
];

export const ZIELE: NavZiel[] = [
  { href: '/', label: t('Übersicht'), kurz: t('Start'), hinweis: t('Alles Wichtige von heute'), icon: House, gruppe: 'alltag', leiste: true },
  { href: '/einkaufen', label: t('Einkaufen'), kurz: t('Einkauf'), hinweis: t('Die gemeinsame Liste'), icon: ShoppingCart, gruppe: 'alltag', leiste: true },
  { href: '/essen', label: t('Essensplan'), kurz: t('Essen'), hinweis: t('Was gibt es diese Woche'), icon: UtensilsCrossed, gruppe: 'alltag', leiste: true },
  { href: '/rezepte', label: t('Rezepte'), kurz: t('Rezepte'), hinweis: t('Das Rezeptbuch der Familie'), icon: BookOpen, gruppe: 'alltag', leiste: true, modul: 'rezepte' },
  { href: '/wetter', label: t('Wetter'), kurz: t('Wetter'), hinweis: t('Regen- und Trockenzeiten'), icon: CloudSun, gruppe: 'alltag', leiste: false },
  { href: '/zeiten', label: t('Arbeit & Schule'), kurz: t('Zeiten'), hinweis: t('Wer wann weg ist'), icon: Clock, gruppe: 'alltag', leiste: false },
  { href: '/belohnungen', label: t('Belohnungen'), kurz: t('Belohnung'), hinweis: t('Punkte ausgeben, Wünsche erfüllen'), icon: Gift, gruppe: 'familie', leiste: true },
  { href: '/rangliste', label: t('Rangliste'), kurz: t('Rangliste'), hinweis: t('Wer hat wie viele Punkte'), icon: Trophy, gruppe: 'familie', leiste: false, persoenlich: true },
  { href: '/neuigkeiten', label: t('Neuigkeiten'), kurz: t('News'), hinweis: t('Feeds der Familie'), icon: Newspaper, gruppe: 'familie', leiste: true, modul: 'neuigkeiten' },
  { href: '/fotos', label: t('Fotos'), kurz: t('Fotos'), hinweis: t('Bildschirmschoner und Fotoarchiv'), icon: Images, gruppe: 'familie', leiste: false },
  { href: '/links', label: t('Links'), kurz: t('Links'), hinweis: t('Lesezeichen der Familie'), icon: LinkIcon, gruppe: 'familie', leiste: false, persoenlich: true },
  { href: '/ansicht', label: t('Ansicht anpassen'), kurz: t('Ansicht'), hinweis: t('Fenster ordnen und ausblenden'), icon: LayoutGrid, gruppe: 'ich', leiste: false, persoenlich: true },
  { href: '/settings', label: t('Einstellungen'), kurz: t('Einstellungen'), hinweis: t('Profil, PIN, Design'), icon: Settings, gruppe: 'ich', leiste: false, persoenlich: true },
  { href: '/admin', label: t('Verwaltung'), kurz: t('Verwaltung'), hinweis: t('Personen, Bereiche, Sicherung'), icon: Shield, gruppe: 'verwaltung', leiste: false, nurAdmin: true },
];

/** Die Ziele, die für diese Sitzung gelten. */
export function zieleFuer(geraet: boolean, admin: boolean): NavZiel[] {
  return ZIELE.filter((z) => {
    if (z.nurAdmin && !admin) return false;
    if (z.persoenlich && geraet) return false;
    if (z.modul === 'rezepte' && !einstellungen.rezepte) return false;
    // Die Neuigkeiten: eingeschaltet in der Verwaltung, und nicht für sich
    // ausgeblendet. Das Wandgerät kennt keine persönliche Auswahl.
    if (z.modul === 'neuigkeiten' && !(geraet ? einstellungen.neuigkeitenAn : einstellungen.neuigkeitenSichtbar)) {
      return false;
    }
    return true;
  });
}

/** Die Ziele einer Gruppe, in der Reihenfolge des Menüs. */
export function nachGruppen(ziele: NavZiel[]): { gruppe: { id: NavGruppe; label: string }; ziele: NavZiel[] }[] {
  return GRUPPEN.map((gruppe) => ({ gruppe, ziele: ziele.filter((z) => z.gruppe === gruppe.id) })).filter(
    (g) => g.ziele.length > 0,
  );
}

/** Ist dieser Pfad gerade aktiv? Die Übersicht nur exakt, alles andere samt Unterseiten. */
export function aktiv(pfad: string, href: string): boolean {
  return href === '/' ? pfad === '/' : pfad === href || pfad.startsWith(`${href}/`);
}

class MenuStore {
  offen = $state(false);
}

/** Das Seitenmenü — geöffnet vom Menüknopf oben oder von „Mehr" unten. */
export const menu = new MenuStore();

