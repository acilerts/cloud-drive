## Modeli podataka

Users servis koristi MongoDB za čuvanje korisničkih naloga, tokena i OTP kodova. Podaci su organizovani u tri kolekcije: `users`, `tokens` i `otps`.

### Kolekcija `users`

U ovoj kolekciji čuvaju se podaci o korisničkim nalozima.

| Atribut | Tip | Opis |
|---|---|---|
| `_id` | ObjectID | Jedinstveni identifikator korisnika |
| `firstName` | String | Ime korisnika |
| `lastName` | String | Prezime korisnika |
| `email` | String | Imejl adresa korisnika; jedinstvena i čuva se malim slovima |
| `passwordHash` | String | Heš lozinke generisan pomoću bcrypt-a |
| `role` | String | Uloga korisnika (`user` ili `admin`) |
| `status` | String | Status naloga (`pending` ili `active`) |
| `quotaBytes` | Int64 | Maksimalni prostor koji korisnik može da koristi, izražen u bajtovima |
| `passwordChangedAt` | DateTime | Vreme poslednje promene lozinke |
| `createdAt` | DateTime | Vreme kreiranja korisničkog naloga |
| `failedLogins` | Int | Broj neuspešnih pokušaja prijavljivanja |
| `lockedUntil` | DateTime | Vreme do kojeg je prijavljivanje korisnika blokirano |

### Kolekcija `tokens`

U ovoj kolekciji čuvaju se tokeni povezani sa korisničkim nalozima. Trenutno je definisana vrsta tokena `confirm`, koja se koristi za potvrdu registracije.

| Atribut | Tip | Opis |
|---|---|---|
| `_id` | ObjectID | Jedinstveni identifikator tokena |
| `userId` | ObjectID | Identifikator korisnika kome token pripada |
| `type` | String | Vrsta tokena |
| `tokenHash` | String | Heš tokena generisan pomoću SHA-256 |
| `expiresAt` | DateTime | Vreme isteka tokena |
| `createdAt` | DateTime | Vreme kreiranja tokena |

Tokeni se ne čuvaju u izvornom obliku, već se u bazi čuva njihov heš. Za polje `expiresAt` koristi se TTL indeks radi automatskog uklanjanja isteklih tokena.

### Kolekcija `otps`

U ovoj kolekciji čuvaju se OTP kodovi povezani sa korisničkim nalozima, koji se koriste u procesu dodatne provere prilikom prijavljivanja.

| Atribut | Tip | Opis |
|---|---|---|
| `_id` | ObjectID | Jedinstveni identifikator OTP zapisa |
| `userId` | ObjectID | Identifikator korisnika kome OTP kod pripada |
| `codeHash` | String | Heš OTP koda |
| `attempts` | Int | Broj pokušaja unosa OTP koda |
| `expiresAt` | DateTime | Vreme isteka OTP koda |
| `createdAt` | DateTime | Vreme kreiranja OTP zapisa |

OTP kodovi se ne čuvaju u izvornom obliku, već se čuva njihov heš. Za jednog korisnika predviđen je jedinstveni OTP zapis, a za polje `expiresAt` koristi se TTL indeks.