import { einstellungenApi, neuigkeitenApi } from '$lib/api';
import type { Einstellungen } from '$lib/types';

/**
 * Die Schalter der Familie, einmal geladen und überall gelesen: Menü,
 * Bildschirmschoner, Rezepte, Neuigkeiten. Was eine Person für sich
 * ausblendet, kommt dazu.
 *
 * Bis die Schalter da sind, gilt der Standard des Servers — damit das Menü
 * nicht erst Punkte zeigt und sie wieder wegnimmt.
 */
class EinstellungenStore {
  werte = $state<Einstellungen | null>(null);
  /** Hat diese Person die Neuigkeiten für sich ausgeblendet? */
  neuigkeitenAus = $state(false);
  geladen = $state(false);

  async load(person: boolean) {
    try {
      this.werte = await einstellungenApi.get();
    } catch {
      /* Ohne Antwort bleiben die Standardwerte. */
    }
    if (person) {
      try {
        const { value } = await neuigkeitenApi.ausgeblendet();
        this.neuigkeitenAus = value === '1' || value === true;
      } catch {
        /* Nicht gesetzt: sichtbar. */
      }
    }
    this.geladen = true;
  }

  /** Nach einer Änderung in der Verwaltung. */
  uebernehmen(werte: Einstellungen) {
    this.werte = werte;
  }

  get rezepte(): boolean {
    return this.werte ? this.werte['modul.rezepte'] === '1' : true;
  }
  /** Die Neuigkeiten gibt es überhaupt (Verwaltung hat sie eingeschaltet). */
  get neuigkeitenAn(): boolean {
    return this.werte?.['modul.neuigkeiten'] === '1';
  }
  get neuigkeitenPersoenlich(): boolean {
    return this.neuigkeitenAn && this.werte?.['modul.neuigkeiten_persoenlich'] === '1';
  }
  /** … und diese Person will sie sehen. */
  get neuigkeitenSichtbar(): boolean {
    return this.neuigkeitenAn && !this.neuigkeitenAus;
  }

  async setzeNeuigkeitenAus(aus: boolean) {
    await neuigkeitenApi.ausblenden(aus);
    this.neuigkeitenAus = aus;
  }
}

export const einstellungen = new EinstellungenStore();
