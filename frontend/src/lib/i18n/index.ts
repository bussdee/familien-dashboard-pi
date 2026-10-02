import { browser } from '$app/environment';
import { de as deLocale, enGB } from 'date-fns/locale';
import { en } from './en';

/**
 * Sprache der Oberfläche: Deutsch oder Englisch.
 *
 * Der deutsche Text ist zugleich der Schlüssel. t('Aufgaben') liefert auf
 * Deutsch „Aufgaben" und auf Englisch, was in en.ts dafür steht. So bleibt
 * der Code lesbar wie bisher, und eine fehlende Übersetzung fällt nicht als
 * leere Stelle auf, sondern als deutsches Wort — das Prüfskript
 * (scripts/i18n-check.mjs) meldet sie beim Bauen.
 *
 * Die Sprache gilt pro Gerät, wie hell und dunkel. Ein Wechsel lädt die
 * Seite einmal neu: Dann stimmt jeder Text, auch der in Listen, die beim
 * Start einmal gebaut werden — ohne dass jede Kachel auf den Wechsel achten
 * muss.
 */
export type Sprache = 'de' | 'en';

export const SPRACHEN: { value: Sprache; label: string }[] = [
  { value: 'de', label: 'Deutsch' },
  { value: 'en', label: 'English' },
];

function anfangsSprache(): Sprache {
  if (!browser) return 'de';
  try {
    const gespeichert = localStorage.getItem('sprache');
    if (gespeichert === 'de' || gespeichert === 'en') return gespeichert;
  } catch {
    /* private mode */
  }
  // Ohne Wahl entscheidet das Gerät: Ein deutsches Tablet bleibt deutsch.
  return navigator.language?.toLowerCase().startsWith('de') ? 'de' : 'en';
}

export const sprache: Sprache = anfangsSprache();

if (browser) document.documentElement.lang = sprache;

export function setzeSprache(neu: Sprache) {
  if (neu === sprache) return;
  try {
    localStorage.setItem('sprache', neu);
  } catch {
    /* private mode */
  }
  location.reload();
}

type Werte = Record<string, unknown> | unknown[];

/**
 * Übersetzt einen Text. Platzhalter stehen in geschweiften Klammern und
 * werden nach der Übersetzung gefüllt: t('Noch {n} Punkte', { n: 35 }) oder
 * t('{0} erledigt', [titel]).
 */
export function t(text: string, werte?: Werte): string {
  const uebersetzt = sprache === 'en' ? (en[text] ?? text) : text;
  if (!werte) return uebersetzt;
  return uebersetzt.replace(/\{(\w+)\}/g, (ganz, name: string) => {
    const wert = Array.isArray(werte) ? werte[Number(name)] : werte[name];
    return wert === undefined ? ganz : String(wert);
  });
}

/** Für date-fns: format(datum, 'EEEE', { locale: dfLocale }). */
export const dfLocale = sprache === 'en' ? enGB : deLocale;

/** Für toLocaleDateString und Intl. */
export const intlLocale = sprache === 'en' ? 'en-GB' : 'de-DE';

/**
 * Texte, die der Server schickt, sind deutsch — Fehlermeldungen, Namen der
 * Level und Abzeichen, Wetterbeschreibungen. Feste stehen in en.ts; solche
 * mit Zahlen oder Namen darin werden hier über Muster übersetzt.
 */
const MUSTER: [RegExp, string][] = [
  [/^Dafür fehlen noch (\d+) Punkte$/, '$1 more points needed'],
  [/^Zu viele Fehlversuche\. Gesperrt bis (.+)$/, 'Too many attempts. Locked until $1'],
  [/^(\d+) Tage Serie$/, '$1-day streak'],
  [/^Einkauf erledigt \((\d+) Artikel\)$/, 'Shopping done ($1 items)'],
  [/^„(.+)“ ist bereits erledigt\.$/, '“$1” is already done.'],
  [/^„(.+)“ hat (.+) schon erledigt\. Wieder dran ist die Aufgabe morgen\.$/, '$2 already did “$1”. It is due again tomorrow.'],
  [/^„(.+)“ hat (.+) schon erledigt\. Wieder dran ist die Aufgabe am (.+)\.$/, '$2 already did “$1”. It is due again on $3.'],
  [/^„(.+)“ ist noch nicht wieder dran — erst morgen\.$/, '“$1” is not due again until tomorrow.'],
  [/^„(.+)“ ist noch nicht wieder dran — erst am (.+)\.$/, '“$1” is not due again until $2.'],
  [/^Unbekannte Zeitzone: (.+)$/, 'Unknown time zone: $1'],
  [/^HTTP (\d+) \(erwartet (\d+)\)$/, 'HTTP $1 (expected $2)'],
];

export function tServer(text: string): string {
  if (sprache !== 'en' || !text) return text;
  if (en[text]) return en[text];
  for (const [muster, ersatz] of MUSTER) {
    if (muster.test(text)) return text.replace(muster, ersatz);
  }
  // „Prüf-Adresse: keine gültige Adresse", „2026-09-10: Uhrzeit muss …":
  // Der Server setzt manche Meldungen aus zwei Teilen zusammen.
  const teile = text.match(/^(.+?): (.+)$/);
  if (teile && (en[teile[1]] || en[teile[2]])) {
    return `${en[teile[1]] ?? teile[1]}: ${en[teile[2]] ?? teile[2]}`;
  }
  return text;
}

/** „September 2026" aus „2026-09" — in der Sprache der Oberfläche. */
export function monatsName(monat: string): string {
  const [j, m] = monat.split('-').map(Number);
  if (!j || !m) return monat;
  return new Date(j, m - 1, 1).toLocaleDateString(intlLocale, { month: 'long', year: 'numeric' });
}
