# API Gateway – Arhitektura i rute

API Gateway predstavlja centralnu ulaznu tačku u sistem na portu `8080` (prema zahtevu 2.2). Svi klijentski zahtevi upućuju se na ovu adresu.

## 1. Serviranje klijentske aplikacije (Frontend)

Gateway direktno servira statičke datoteke iz `client/` direktorijuma:

- `GET /` – automatski preusmerava na `/login.html`
- `GET /login.html` – stranica za prijavu i unos OTP koda
- `GET /register.html` – stranica za registraciju korisnika
- `GET /confirm.html` – stranica za potvrdu registracije tokenom
- `GET /index.html` – glavni radni prostor Cloud Drive-a
- `GET /css/*`, `GET /js/*` – stilovi i skripte

## 2. Rutiranje ka mikroservisima (Reverse Proxy)

Zahtevi ka API-ju se transparentno prosleđuju odgovarajućim servisima:

| Putanja | Odredišni servis | Opis |
|---|---|---|
| `/register` | Users (`8081`) | Registracija naloga |
| `/confirm` | Users (`8081`) | Potvrda naloga tokenom |
| `/login` | Users (`8081`) | Prvi korak prijave (email + lozinka) |
| `/login/verify` | Users (`8081`) | Drugi korak prijave (verifikacija OTP koda) |
| `/password-*` | Users (`8081`) | Promena i reset lozinke |
| `/files/*` | Files (`8082`) | Upravljanje datotekama |
| `/folders/*` | Files (`8082`) | Upravljanje hijerarhijom foldera |
| `/notifications/*` | Notifications (`8084`) | Pristup notifikacijama |

## 3. Nadzor servisa (Health check)

- **`GET /health`**
- **Odgovor (200 OK):**
```json
{
  "service": "gateway",
  "status": "ok"
}
```
