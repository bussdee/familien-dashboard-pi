import { t } from '$lib/i18n';
import {
  Clock, CloudSun, Gift, House, LayoutGrid, Link as LinkIcon, Settings, Shield,
  ShoppingCart, Trophy, UtensilsCrossed,
} from 'lucide-svelte';
import type { ComponentType } from 'svelte';

/**
 * Alle Ziele der App an einer Stelle. Kopfleiste, Menü und die Leiste am
 * unteren Rand lesen von hier — vorher stand die Liste zweimal im Code und
 * lief bei jedem neuen Punkt auseinander.
 */
export interface NavZiel {
  href: string;
  label: string;
  /** Kurzform für die schmale Leiste unten. */
  kurz: string;
  icon: ComponentType;
  /** Steht in der Kopfleiste am grossen Bildschirm. */
  leiste: boolean;
  /** Nur mit persönlicher Anmeldung sinnvoll — am Wandgerät ausgeblendet. */
  persoenlich?: boolean;
  nurAdmin?: boolean;
}

export const ZIELE: NavZiel[] = [
  { href: '/', label: t('Übersicht'), kurz: t('Start'), icon: House, leiste: true },
  { href: '/einkaufen', label: t('Einkaufen'), kurz: t('Einkauf'), icon: ShoppingCart, leiste: true },
  { href: '/essen', label: t('Essensplan'), kurz: t('Essen'), icon: UtensilsCrossed, leiste: true },
  { href: '/wetter', label: t('Wetter'), kurz: t('Wetter'), icon: CloudSun, leiste: true },
  { href: '/zeiten', label: t('Arbeit & Schule'), kurz: t('Zeiten'), icon: Clock, leiste: true },
  { href: '/rangliste', label: t('Rangliste'), kurz: t('Rangliste'), icon: Trophy, leiste: true, persoenlich: true },
  { href: '/belohnungen', label: t('Belohnungen'), kurz: t('Belohnung'), icon: Gift, leiste: true },
  { href: '/links', label: t('Links'), kurz: t('Links'), icon: LinkIcon, leiste: false, persoenlich: true },
  { href: '/ansicht', label: t('Ansicht anpassen'), kurz: t('Ansicht'), icon: LayoutGrid, leiste: false, persoenlich: true },
  { href: '/settings', label: t('Einstellungen'), kurz: t('Einstellungen'), icon: Settings, leiste: false, persoenlich: true },
  { href: '/admin', label: t('Verwaltung'), kurz: t('Verwaltung'), icon: Shield, leiste: false, nurAdmin: true },
];

/** Die Ziele, die für diese Sitzung gelten. */
export function zieleFuer(geraet: boolean, admin: boolean): NavZiel[] {
  return ZIELE.filter((z) => {
    if (z.nurAdmin) return admin;
    if (z.persoenlich) return !geraet;
    return true;
  });
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

