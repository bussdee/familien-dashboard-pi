# Sicherheit

## Was dieses Projekt schützt — und was nicht

Das Familien Dashboard ist für das eigene Heimnetz gebaut. Wer schon im
WLAN ist, gehört nach diesem Modell zur Familie oder ist zu Gast. Daraus
folgt eine ehrliche Einordnung:

**Was da ist**

- Anmeldung mit vierstelliger PIN, gespeichert als argon2id-Hash
- Sitzung in einem HttpOnly-Cookie, signiert mit einem Schlüssel, der bei
  der Installation zufällig erzeugt wird
- Sperre nach fünf Fehlversuchen für fünf Minuten
- Container laufen ohne Root, mit schreibgeschütztem Dateisystem und ohne
  Zugriff auf den Docker-Socket
- Nur zwei Ports nach außen: 8088 und 8443
- Keine Cloud, keine Telemetrie, keine Konten bei Dritten. Nach außen geht
  eine einzige Anfrage: das Wetter bei Open-Meteo.

**Was ausdrücklich nicht da ist**

- Eine vierstellige PIN ist kein Passwort. Sie hält kleine Geschwister ab,
  keinen Angreifer.
- Keine Verschlüsselung der Daten auf der Festplatte
- Keine Zwei-Faktor-Anmeldung, keine Rechteverwaltung über Admin und
  Mitglied hinaus
- Das mitgelieferte HTTPS-Zertifikat ist selbstsigniert

## Nicht ins Internet stellen

Dieses Projekt gehört **nicht** ins offene Netz — keine Portweiterleitung
im Router, kein öffentlicher Reverse Proxy, keine Domain darauf. Es ist
für diesen Zweck nicht gebaut und wird es auch nicht sein.

Wer von unterwegs an die Familienliste möchte, nimmt ein VPN ins eigene
Heimnetz (WireGuard, oder was der Router mitbringt). Dann gilt wieder das
Modell, für das dieses Dashboard gemacht ist.

## Eine Lücke melden

Wenn du etwas gefunden hast, das über die oben genannten Grenzen
hinausgeht — etwa Zugriff auf fremde Daten ohne Anmeldung, oder ein Weg,
die PIN-Sperre zu umgehen:

Bitte **kein öffentliches Issue**. Nutze stattdessen
[Security Advisories](https://github.com/bussdee/familien-dashboard-pi/security/advisories/new)
auf GitHub, oder schreib über [familienfabrik.at](https://familienfabrik.at).

Das hier ist ein Feierabendprojekt einer Einzelperson. Ich melde mich, so
schnell ich kann, kann aber keine feste Frist zusagen. Gefundene Lücken
werden im [CHANGELOG.md](CHANGELOG.md) genannt, sobald sie behoben sind.

## Unterstützte Versionen

Es gibt nur eine gepflegte Fassung: die aktuelle. Ältere Versionen
bekommen keine Nachbesserungen — bitte auf den neuesten Stand gehen.
