# API ugovor – Files servis

Files servis upravlja metapodacima o fajlovima, njihovim verzijama i folderima. Metapodaci se čuvaju u MongoDB bazi, dok se sadržaj fajlova čuva u MinIO skladištu.

Servis omogućava:
- Kreiranje foldera i podfoldera.
- Izlistavanje foldera.
- Otpremanje fajlova.
- Otpremanje novih verzija postojećih fajlova.

## Autentifikacija

Svi endpoint-i zahtevaju validan JWT token registrovanog korisnika sa ulogom `user`.

Token se šalje kroz HTTP zaglavlje:

```http
Authorization: Bearer <token>
