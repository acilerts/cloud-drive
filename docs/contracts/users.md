# Users servis – API ugovor

Dok se ne uvede API Gateway, Users servisu pristupamo direktno preko adrese `http://localhost:8081`.

## `POST /register` – Registracija korisnika

- **Pristup:** nije potrebna prijava.
- **Telo zahteva:**

```json
{
  "firstName": "Ana",
  "lastName": "Anić",
  "email": "ana@example.com",
  "password": "Abcdef1!xy",
  "passwordRepeat": "Abcdef1!xy"
}
```

- **201 Created:** registracija je uspešno završena i korisniku se šalje imejl sa linkom za potvrdu naloga.

```json
{
  "message": "registration successful, check your email to confirm"
}
```

**Moguće greške:**

| Status | Opis |
|---|---|
| 400 | Podaci nisu ispravni. Odgovor sadrži poruke o greškama za konkretna polja. |
| 409 | Korisnik sa unetom imejl adresom već postoji. |
| 503 | Imejl nije mogao da bude poslat. U tom slučaju nalog se ne kreira. |

Primer odgovora za grešku 400:

```json
{
  "error": "validation failed",
  "fields": {
    "polje": "poruka"
  }
}
```

## `POST /confirm` – Potvrda naloga

Ovaj endpoint služi za potvrdu registracije pomoću tokena koji korisnik dobija u imejlu.

- **Pristup:** nije potrebna prijava.
- **Telo zahteva:**

```json
{
  "token": "<token iz linka>"
}
```

- **200 OK:** nalog je uspešno potvrđen.

```json
{
  "message": "account confirmed"
}
```

- **400 Bad Request:** token nije ispravan, istekao je ili je već iskorišćen.

Token se šalje u telu POST zahteva, a ne direktno kroz URL. Link iz imejla vodi do klijentske stranice (`/confirm.html?token=...`), koja zatim šalje token ovom endpoint-u.

## Modeli podataka

### Kolekcija `users`

U ovoj kolekciji čuvaju se podaci o korisnicima:

- `_id` – jedinstveni ID korisnika
- `firstName` – ime
- `lastName` – prezime
- `email` – imejl adresa (jedinstvena, čuva se malim slovima)
- `passwordHash` – heš lozinke, generisan pomoću bcrypt-a
- `role` – uloga korisnika (`user` ili `admin`)
- `status` – status naloga (`pending` ili `active`)
- `quotaBytes` – prostor koji korisnik može da koristi
- `passwordChangedAt` – vreme poslednje promene lozinke
- `createdAt` – vreme kreiranja naloga

### Kolekcija `tokens`

U ovoj kolekciji čuvaju se tokeni koji se koriste za potvrdu naloga i druge predviđene funkcionalnosti:

- `_id` – ID tokena
- `userId` – ID korisnika kome token pripada
- `type` – vrsta tokena
- `tokenHash` – heš tokena, generisan pomoću SHA-256
- `expiresAt` – vreme isteka tokena (koristi se TTL indeks)
- `createdAt` – vreme kreiranja tokena