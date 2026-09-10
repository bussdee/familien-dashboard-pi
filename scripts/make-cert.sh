#!/usr/bin/env bash
# Erzeugt ein Zertifikat, dem eure Geräte trauen können.
#
# WARUM DAS NÖTIG IST
#
# Chrome erlaubt „App installieren", Vollbild ohne Adressleiste und den
# Offline-Modus nur auf einer sicheren Herkunft. Über http:// gibt es das
# nicht, und über https:// mit Traefiks eingebautem Platzhalter-Zertifikat
# auch nicht: Die Warnung lässt sich zwar wegklicken, aber der Browser
# behandelt die Seite danach trotzdem nicht als vertrauenswürdig.
#
# WAS DIESES SKRIPT MACHT
#
# Es legt eine kleine eigene Zertifizierungsstelle an — eine Datei, die sagt
# „diesen Zertifikaten glaube ich" — und stellt damit ein Zertifikat für die
# Adresse eures Dashboards aus. Die Stelle wird einmal pro Gerät eingerichtet,
# danach ist Ruhe.
#
# Die Stelle gilt zehn Jahre, das Zertifikat zwei. Beides bleibt in diesem
# Ordner und wird NICHT mit ins Git aufgenommen.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CERTS="$ROOT/traefik/certs"

# Die Adresse, unter der ihr das Dashboard aufruft. Mehrere sind erlaubt:
#   bash scripts/make-cert.sh 192.168.1.20 dashboard.fritz.box
ADRESSEN=("$@")
if [ ${#ADRESSEN[@]} -eq 0 ]; then
  # Ohne Angabe: die LAN-Adresse dieses Rechners raten und nachfragen.
  GERATEN="$(hostname -I 2>/dev/null | awk '{print $1}')"
  echo ""
  echo "Für welche Adresse soll das Zertifikat gelten?"
  echo "Das ist die Adresse, die ihr im Browser eintippt — ohne https:// und"
  echo "ohne Port."
  echo ""
  read -r -p "Adresse [${GERATEN:-192.168.1.20}]: " EINGABE
  ADRESSEN=("${EINGABE:-${GERATEN:-192.168.1.20}}")
fi

command -v openssl >/dev/null || { echo "❌ openssl fehlt."; exit 1; }

mkdir -p "$CERTS"
chmod 700 "$CERTS"

# ── Die Zertifizierungsstelle ────────────────────────────────────────────────
# Sie wird nur einmal erzeugt. Wird sie neu erzeugt, muss sie auf JEDEM Gerät
# neu eingerichtet werden — deshalb bleibt eine vorhandene unangetastet.
if [ -f "$CERTS/familie-ca.crt" ]; then
  echo "ℹ️  Zertifizierungsstelle besteht bereits — unverändert gelassen."
else
  echo "▶ Erzeuge die Zertifizierungsstelle der Familie..."
  openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
    -keyout "$CERTS/familie-ca.key" \
    -out "$CERTS/familie-ca.crt" \
    -subj "/CN=Familien Dashboard/O=Familien Dashboard" \
    -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null
  chmod 600 "$CERTS/familie-ca.key"
  echo "  + traefik/certs/familie-ca.crt"
fi

# ── Das Zertifikat für das Dashboard ─────────────────────────────────────────
# Jede Adresse kommt als SAN hinein. Ohne SAN akzeptiert kein heutiger Browser
# ein Zertifikat, egal wie sehr er der Stelle traut — der Name im alten
# CN-Feld wird seit Jahren ignoriert.
SAN=""
for a in "${ADRESSEN[@]}"; do
  if [[ "$a" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    SAN="${SAN}IP:${a},"
  else
    SAN="${SAN}DNS:${a},"
  fi
done
SAN="${SAN}DNS:localhost,IP:127.0.0.1"

echo "▶ Stelle das Zertifikat aus für: ${ADRESSEN[*]}"
openssl req -newkey rsa:2048 -sha256 -nodes \
  -keyout "$CERTS/dashboard.key" \
  -out "$CERTS/dashboard.csr" \
  -subj "/CN=${ADRESSEN[0]}" 2>/dev/null

openssl x509 -req -in "$CERTS/dashboard.csr" \
  -CA "$CERTS/familie-ca.crt" -CAkey "$CERTS/familie-ca.key" -CAcreateserial \
  -out "$CERTS/dashboard.crt" -days 730 -sha256 \
  -extfile <(printf 'subjectAltName=%s\nextendedKeyUsage=serverAuth\nbasicConstraints=CA:FALSE\n' "$SAN") \
  2>/dev/null

rm -f "$CERTS/dashboard.csr"
chmod 600 "$CERTS/dashboard.key"

# ── Traefik sagen, dass es das nehmen soll ───────────────────────────────────
cat > "$ROOT/traefik/dynamic/certs.yml" <<YAML
# Von scripts/make-cert.sh erzeugt — nicht von Hand ändern.
#
# Liegt diese Datei nicht da, nimmt Traefik sein eingebautes
# Platzhalter-Zertifikat. Dann funktioniert HTTPS zwar, aber jeder Browser
# warnt, und installieren lässt sich die App nicht.
tls:
  stores:
    default:
      defaultCertificate:
        certFile: /etc/traefik/certs/dashboard.crt
        keyFile: /etc/traefik/certs/dashboard.key
  certificates:
    - certFile: /etc/traefik/certs/dashboard.crt
      keyFile: /etc/traefik/certs/dashboard.key
YAML

echo ""
echo "✅ Fertig."
openssl x509 -in "$CERTS/dashboard.crt" -noout -subject -dates \
  -ext subjectAltName | sed 's/^/   /'
echo ""
echo "Jetzt neu starten, damit Traefik das Zertifikat nimmt:"
echo ""
echo "    make up"
echo ""
echo "Danach EINMAL PRO GERÄT die Zertifizierungsstelle einrichten."
echo "Die Datei dafür ist:"
echo ""
echo "    traefik/certs/familie-ca.crt"
echo ""
echo "  Android:  Datei aufs Handy kopieren, dann"
echo "            Einstellungen → Sicherheit → Verschlüsselung & Anmeldedaten"
echo "            → Zertifikat installieren → CA-Zertifikat → Trotzdem installieren"
echo ""
echo "  iPhone:   Datei per AirDrop oder Mail aufs Gerät, öffnen, dann"
echo "            Einstellungen → Profil geladen → Installieren."
echo "            DANACH ZUSÄTZLICH: Einstellungen → Allgemein → Info →"
echo "            Zertifikatsvertrauenseinstellungen → Schalter umlegen."
echo "            Ohne diesen zweiten Schritt bleibt die Warnung."
echo ""
echo "  Windows:  Doppelklick → Zertifikat installieren → Lokaler Computer"
echo "            → Alle Zertifikate in folgendem Speicher →"
echo "            Vertrauenswürdige Stammzertifizierungsstellen"
echo ""
echo "  Linux:    sudo cp traefik/certs/familie-ca.crt \\"
echo "              /usr/local/share/ca-certificates/familien-dashboard.crt"
echo "            sudo update-ca-certificates"
echo ""
echo "  Firefox bringt einen eigenen Speicher mit: Einstellungen →"
echo "  Zertifikate → Zertifikate anzeigen → Importieren."
echo ""
echo "⚠️  Der Schlüssel der Stelle (traefik/certs/familie-ca.key) darf das Haus"
echo "    nicht verlassen. Wer ihn hat, kann Zertifikate ausstellen, denen eure"
echo "    Geräte glauben."
