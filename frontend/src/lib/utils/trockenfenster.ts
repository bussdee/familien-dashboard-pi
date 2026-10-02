import { t, dfLocale, intlLocale } from '$lib/i18n';
/**
 * Trockenfenster — die Wetterfrage, die am Küchentisch gestellt wird.
 *
 * Das Backend rechnet die Fenster aus (nur zwischen Sonnenauf- und
 * -untergang, zusammenhängende trockene Stunden, nichts unter anderthalb
 * Stunden). Hier wird daraus ein Satz, den man im Vorbeigehen liest.
 *
 * Bewusst keine zweite Regel-Ebene: Was hier steht, formuliert nur — es
 * entscheidet nicht, was ein Fenster ist. Sonst hätten Übersicht und
 * Wetterseite irgendwann zwei verschiedene Meinungen.
 */
import { format, isToday, isTomorrow, parseISO } from 'date-fns';
import type { WeatherData, WeatherDay, WeatherWindow } from '$lib/types';

export const uhr = (iso: string): string => {
  try {
    return format(parseISO(iso), 'HH:mm');
  } catch {
    return '—';
  }
};

/**
 * Ein Zeitpunkt mit Tag, wo der Tag nicht selbstverständlich ist.
 *
 * „Regen ab 20:30" unter einem Satz über morgen liest sich wie heute Abend.
 * Steht der Tag dabei, ist die Frage erledigt, bevor sie entsteht.
 */
export function zeitpunkt(iso: string): string {
  try {
    const zeit = parseISO(iso);
    if (isToday(zeit)) return format(zeit, 'HH:mm');
    if (isTomorrow(zeit)) return t('morgen {0}', [format(zeit, 'HH:mm')]);
    return format(zeit, 'EEEE HH:mm', { locale: dfLocale });
  } catch {
    return '—';
  }
}

/** Ein Fenster weiss selbst nicht, an welchem Tag es liegt — hier schon. */
export interface FensterAmTag {
  fenster: WeatherWindow;
  tag: WeatherDay;
  /** 0 = heute, 1 = morgen, … — der Index in der Vorhersage. */
  tagIndex: number;
}

/** Alle Fenster der Vorhersage, in zeitlicher Reihenfolge. */
export function alleFenster(weather: WeatherData | null): FensterAmTag[] {
  if (!weather) return [];
  const out: FensterAmTag[] = [];
  weather.forecast.forEach((tag, tagIndex) => {
    (tag.windows ?? []).forEach((fenster) => out.push({ fenster, tag, tagIndex }));
  });
  return out;
}

/** Das nächste Fenster — das laufende, sonst das kommende. */
export function naechstesFenster(weather: WeatherData | null): FensterAmTag | null {
  return alleFenster(weather)[0] ?? null;
}

/** „2 Std.", „1,5 Std." — die halbe Stunde nur, wenn es sie gibt. */
export function dauer(stunden: number): string {
  const text = stunden.toLocaleString(intlLocale, { maximumFractionDigits: 1 });
  return t('{0} Std.', [text]);
}

/** „heute", „morgen", „am Donnerstag" — als Einschub in einem Satz. */
export function tagWort(eintrag: FensterAmTag): string {
  if (eintrag.tagIndex === 0) return t('heute');
  if (eintrag.tagIndex === 1) return t('morgen');
  try {
    return t('am {0}', [format(parseISO(eintrag.tag.date), 'EEEE', { locale: dfLocale })]);
  } catch {
    return '';
  }
}

/**
 * Der Satz für die Übersicht. Kurz genug für den Flur, genau genug zum Planen.
 *
 * Ein laufendes Fenster nennt nur das Ende: Dass es jetzt trocken ist, sieht
 * man aus dem Fenster — wissen will man, wie lange das noch gilt.
 */
export function fensterSatz(eintrag: FensterAmTag | null): string {
  if (!eintrag) return t('Kein trockenes Fenster in Sicht');
  const { fenster } = eintrag;
  if (fenster.now) return t('Trocken bis {0}', [uhr(fenster.to)]);
  const wann = tagWort(eintrag);
  const zeitraum = t('{0} bis {1}', [uhr(fenster.from), uhr(fenster.to)]);
  return eintrag.tagIndex === 0 ? t('Trocken {0}', [zeitraum]) : t('Trocken {0} {1}', [wann, zeitraum]);
}

/**
 * Die Beschreibung darunter: was einen dort erwartet. Sonne und Temperatur
 * entscheiden nicht über das Fenster, sie helfen bei der Entscheidung, ob
 * Jacke oder Sonnencreme mitkommt.
 */
export function fensterBeschreibung(fenster: WeatherWindow): string {
  // Ohne die Dauer: Die steht in der Zeile daneben, und zweimal dieselbe
  // Zahl liest sich wie ein Fehler.
  const teile = [fenster.sunny ? t('sonnig') : fenster.cloud_cover >= 80 ? t('bedeckt') : t('bewölkt')];
  const ab = Math.round(fenster.feels_like_min);
  const bis = Math.round(fenster.feels_like_max);
  teile.push(ab === bis ? t('gefühlt {0}°', [ab]) : t('gefühlt {0}–{1}°', [ab, bis]));
  return teile.join(' · ');
}
