#!/usr/bin/env node
/**
 * Prüft, ob jeder Text der Oberfläche eine englische Übersetzung hat.
 *
 * Gesammelt werden:
 *   - alle t('…')-Aufrufe im Frontend (src/**),
 *   - die festen Texte, die der Go-Server schickt: Fehlermeldungen,
 *     Namen der Level und Abzeichen, Wetterbeschreibungen.
 *
 * Fehlt etwas in src/lib/i18n/en.ts, bricht das Skript mit einer Liste ab.
 * Läuft mit `npm run check` und damit in der CI.
 *
 *   node scripts/i18n-check.mjs            prüfen
 *   node scripts/i18n-check.mjs --liste    fehlende als Vorlage ausgeben
 *   node scripts/i18n-check.mjs --unbenutzt  Übersetzungen ohne Verwendung
 */
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const hier = dirname(fileURLToPath(import.meta.url));
const frontend = join(hier, '..');
const backend = join(frontend, '..', 'backend');

function dateien(ordner, endungen) {
  const out = [];
  for (const name of readdirSync(ordner)) {
    const pfad = join(ordner, name);
    if (statSync(pfad).isDirectory()) {
      if (name === 'node_modules' || name.startsWith('.')) continue;
      out.push(...dateien(pfad, endungen));
    } else if (endungen.some((e) => name.endsWith(e))) {
      out.push(pfad);
    }
  }
  return out;
}

const entschluesseln = (s) => s.replace(/\\(.)/g, '$1');

// ---- Frontend
const schluessel = new Map();
for (const pfad of dateien(join(frontend, 'src'), ['.svelte', '.ts'])) {
  if (pfad.includes(`${join('lib', 'i18n')}`)) continue;
  const src = readFileSync(pfad, 'utf8');
  for (const m of src.matchAll(/\bt\(\s*'((?:[^'\\]|\\.)*)'/g)) {
    schluessel.set(entschluesseln(m[1]), pfad.replace(frontend + '/', ''));
  }
}

// ---- Server (nur, wenn das Backend daneben liegt — im Frontend-Container nicht)
const server = new Map();
const INTERN = new Set(['falsche pin', 'invalid token', 'kein elternteil', 'unbekannt']);
if (existsSync(backend)) {
  for (const pfad of dateien(join(backend, 'internal'), ['.go'])) {
    if (pfad.endsWith('_test.go')) continue;
    const src = readFileSync(pfad, 'utf8');
    const kurz = pfad.replace(join(backend, '..') + '/', '');
    // Fehlermeldungen mit festem Text
    for (const m of src.matchAll(/[hH][tT][tT][pP]Error\(w,\s*[^,]+,\s*"((?:[^"\\]|\\.)*)"\s*\)/g)) {
      server.set(entschluesseln(m[1]), kurz);
    }
    // Fehlermeldungen, die von Hand als JSON geschrieben werden
    for (const m of src.matchAll(/"message":\s*"((?:[^"\\]|\\.)*)"/g)) {
      server.set(entschluesseln(m[1]), kurz);
    }
    // Fehler aus Prüffunktionen, die als err.Error() beim Nutzer ankommen.
    // Konfigurationsfehler (mit _) und interne Kennungen bleiben draussen.
    for (const m of src.matchAll(/(?:fmt\.Errorf|errors\.New)\("([^"%]+)"\)/g)) {
      if (!m[1].includes('_') && !INTERN.has(m[1])) server.set(m[1], kurz);
    }
    // Kurze Meldungen aus Hilfsfunktionen (z. B. warum ein Gerät nicht antwortet)
    for (const m of src.matchAll(/return "([A-ZÄÖÜ][^"%]* [^"%]*)"/g)) server.set(m[1], kurz);
    // Abzeichen: add("id", "emoji", "Label", "Beschreibung")
    for (const m of src.matchAll(/\badd\("[^"]+",\s*"[^"]+",\s*"([^"]+)",\s*"([^"]+)"\)/g)) {
      server.set(m[1], kurz);
      server.set(m[2], kurz);
    }
    // Levelnamen
    const level = src.match(/names := \[\]string\{([^}]+)\}/);
    if (level) for (const m of level[1].matchAll(/"([^"]+)"/g)) server.set(m[1], kurz);
    // Wetterbeschreibungen
    const wetter = src.match(/descriptions := map\[int\]string\{([^}]+)\}/);
    if (wetter) for (const m of wetter[1].matchAll(/\d+:\s*"([^"]+)"/g)) server.set(m[1], kurz);
  }
  // Einzelne Wörter, die der Server in den Verlauf oder in Gerätemeldungen schreibt
  for (const s of ['Bonus', 'Abzug', 'Korrektur', 'Unbekannt', 'Zeitüberschreitung']) server.set(s, 'backend');
}

// ---- Abgleich
const en = readFileSync(join(frontend, 'src', 'lib', 'i18n', 'en.ts'), 'utf8');
const vorhanden = new Set();
for (const m of en.matchAll(/^\s*'((?:[^'\\]|\\.)*)'\s*:/gm)) vorhanden.add(entschluesseln(m[1]));
for (const m of en.matchAll(/^\s*"((?:[^"\\]|\\.)*)"\s*:/gm)) vorhanden.add(entschluesseln(m[1]));

// Rein dynamische oder sprachneutrale Texte brauchen keinen Eintrag.
const neutral = (s) => !/[A-Za-zÄÖÜäöüß]{2,}/.test(s.replace(/\{\w+\}/g, ''));

const fehlend = [];
for (const [k, wo] of [...schluessel, ...server]) {
  if (!vorhanden.has(k) && !neutral(k)) fehlend.push([k, wo]);
}

if (process.argv.includes('--unbenutzt')) {
  // Übersetzungen, die niemand mehr abruft. Kategorien der Einkaufsliste
  // stehen als Daten in der Datenbank und tauchen hier zu Unrecht auf.
  for (const k of vorhanden) if (!schluessel.has(k) && !server.has(k)) console.log(`  ${JSON.stringify(k)}`);
  process.exit(0);
}

if (process.argv.includes('--liste')) {
  for (const [k] of fehlend) console.log(`  ${JSON.stringify(k)}: ${JSON.stringify(k)},`);
  process.exit(0);
}

if (fehlend.length > 0) {
  console.error(`\n❌ ${fehlend.length} Texte ohne englische Übersetzung (src/lib/i18n/en.ts):\n`);
  for (const [k, wo] of fehlend) console.error(`   ${wo}: ${JSON.stringify(k)}`);
  console.error('');
  process.exit(1);
}
console.log(`✅ Übersetzungen vollständig — ${schluessel.size} Texte der Oberfläche, ${server.size} vom Server.`);
