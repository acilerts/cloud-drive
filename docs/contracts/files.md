# API ugovor – Files servis

Files servis upravlja metapodacima o fajlovima i folderima. Metapodaci se čuvaju u MongoDB bazi, dok se sadržaj fajlova čuva u MinIO skladištu.

Trenutno implementirani endpoint-i odnose se na kreiranje i izlistavanje foldera.

## Autentifikacija

Endpoint-i zahtevaju validan JWT token registrovanog korisnika.

Token se šalje kroz HTTP zaglavlje:

```http
Authorization: Bearer <token>
