# API Gateway – Arhitektura i rute

API Gateway predstavlja centralnu ulaznu tačku u sistem na portu `8080` (prema zahtevu 2.2). Podržava HTTPS (TLS 1.2+) u komunikaciji sa klijentskom aplikacijom (zahtev 2.19) i ograničavanje broja zahteva po klijentu radi zaštite od DoS napada (zahtev 2.17).

## 1. Serviranje klijentske aplikacije (Frontend)

Gateway direktno servira statičke datoteke iz `client/` direktorijuma:

- `GET /` – automatski preusmerava na `/login.html`
- `GET /login.html` – stranica za prijavu, OTP verifikaciju, zaboravljenu lozinku i zahtev za magični link
- `GET /register.html` – stranica za registraciju korisnika
- `GET /confirm.html` – stranica za potvrdu registracije tokenom
- `GET /reset.html` – stranica za postavljanje nove lozinke preko reset tokena
- `GET /magic.html` – stranica za prijavu putem magičnog linka
- `GET /index.html` – glavni radni prostor Cloud Drive-a za redovne korisnike
- `GET /admin.html` – administratorski panel za pregled korisnika i zahteva za povećanje kvote
- `GET /css/*`, `GET /js/*` – stilovi i skripte

## 2. Rutiranje ka mikroservisima (Reverse Proxy)

Zahtevi ka API-ju se transparentno prosleđuju odgovarajućim servisima. Interne rute (`/internal/*`) su blokirane na nivou Gateway-a (`404 Not Found`).

| Putanja | Odredišni servis | Opis |
|---|---|---|
| `/register` | Users (`8081`) | Registracija naloga |
| `/confirm` | Users (`8081`) | Potvrda naloga tokenom |
| `/login`, `/login/*` | Users (`8081`) | Prijava i verifikacija OTP koda |
| `/password/*` | Users (`8081`) | Promena (`/password/change`), zahtev (`/password/forgot`) i reset (`/password/reset`) lozinke |
| `/magic/*` | Users (`8081`) | Zahtev (`/magic/request`) i prijava (`/magic/login`) magičnim linkom |
| `/quota/*` | Users (`8081`) | Podnošenje i pregled zahteva za povećanje kvote |
| `/admin/*` | Users (`8081`) | Administratorski pregled korisnika i odlučivanje o zahtevima za kvotu |
| `/files`, `/files/*` | Files (`8082`) | Upravljanje datotekama |
| `/folders`, `/folders/*` | Files (`8082`) | Upravljanje hijerarhijom foldera |
| `/notifications`, `/notifications/*` | Notifications (`8084`) | Pristup notifikacijama |

## 3. Bezbednosne funkcionalnosti

- **HTTPS (TLS):** Aktivira se promenljivom `TLS_ENABLED=true` (automatski self-signed sertifikat za `localhost`) ili prosleđivanjem putanja `TLS_CERT_FILE` i `TLS_KEY_FILE`.
- **Rate limiting (DoS zaštita):** Ograničava broj zahteva po klijentu (`RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`, i stroži limit `AUTH_RATE_LIMIT_RPM` za autentifikacione rute). Pri prekoračenju vraća `429 Too Many Requests` i zaglavlje `Retry-After`.
- **Sigurnosna zaglavlja:** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Content-Security-Policy` i `Strict-Transport-Security` (na HTTPS-u).

## 4. Nadzor servisa (Health check)

- **`GET /health`**
- **Odgovor (200 OK):**
```json
{
  "service": "gateway",
  "status": "ok"
}
```
