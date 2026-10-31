# API ugovori i događaji

Ovaj folder sadrži dogovorene API ugovore između klijentske aplikacije, API Gateway-a i pojedinačnih servisa, kao i formate događaja koje servisi međusobno razmenjuju.

Ugovor definiše način komunikacije: dostupne endpoint-e, podatke koji se šalju, očekivane odgovore i moguće greške.

Izmena postojećeg endpoint-a ili formata događaja mora biti praćena izmenom odgovarajućeg ugovora. Promene koje utiču na druge servise potrebno je prethodno dogovoriti sa članovima tima koji održavaju te servise.

## Organizacija fajlova

Ugovori se vode u zasebnim fajlovima:

- `gateway.md` – endpoint-i dostupni preko API Gateway-a.
- `users.md` – API Users servisa.
- `files.md` – API Files servisa.
- `sharing.md` – API Sharing servisa.
- `notifications.md` – API Notifications servisa.
- `analytics.md` – API Analytics servisa.
- `events.md` – događaji koje servisi objavljuju i obrađuju.

Fajlovi se dopunjuju kada se definiše komunikacija odgovarajućeg servisa. Ne treba dodavati endpoint-e ili događaje koji još nisu dogovoreni.

## Šablon za API endpoint

Svaki endpoint treba da bude opisan sledećim informacijama:

### `METODA /putanja`

- **Servis:** servis koji obrađuje zahtev.
- **Ko poziva:** klijentska aplikacija preko Gateway-a ili drugi servis.
- **Pristup:** neautentifikovan korisnik, registrovani korisnik ili administrator.
- **Opis:** namena endpoint-a.
- **Zahtev:** format i primer JSON tela, ako ga endpoint koristi.

```json
{
  "polje": "vrednost"
}
```

- **Uspešan odgovor:** HTTP status i primer JSON odgovora.

```json
{
  "polje": "vrednost"
}
```

- **Moguće greške:** HTTP statusi i okolnosti u kojima se vraćaju.

| Status | Značenje |
|---|---|
| 400 | Zahtev sadrži neispravne ili nedostajuće podatke |
| 401 | Nedostaje ili nije validna autentifikacija |
| 403 | Korisnik nema potrebna prava pristupa |
| 404 | Traženi resurs ne postoji |
| 409 | Zahtev je u konfliktu sa postojećim stanjem |
| 500 | Interna greška servisa |

Navedeni statusi predstavljaju moguće primere. Za svaki endpoint dokumentuju se samo greške koje se zaista mogu pojaviti.

## Šablon za događaj

Događaji se koriste za asinhronu komunikaciju između servisa. Svaki događaj mora imati jasno definisanog pošiljaoca, primaoce i strukturu podataka.

### `servis.događaj.naziv`

- **Objavljuje:** servis koji generiše događaj.
- **Obrađuju:** servisi koji primaju i obrađuju događaj.
- **Kada se objavljuje:** opis situacije u kojoj događaj nastaje.
- **Sadržaj:** struktura i primer JSON poruke.

```json
{
  "eventId": "unique-event-id",
  "occurredAt": "2026-01-01T12:00:00Z",
  "fileId": "file-id",
  "ownerId": "user-id"
}
```

Polja `eventId` i `occurredAt` služe za identifikaciju događaja i beleženje vremena njegovog nastanka. Ostala polja zavise od konkretnog događaja.

## Pravila za održavanje ugovora

- Ugovor se definiše pre implementacije komunikacije između dva servisa.
- Izmena endpoint-a ili događaja mora biti praćena ažuriranjem odgovarajućeg dokumenta.
- Članovi tima koji održavaju servise povezane izmenom moraju se dogovoriti pre njenog uvođenja.
- PR koji menja način komunikacije između servisa mora sadržati i odgovarajuću izmenu ugovora.
- Primeri zahteva i odgovora moraju odgovarati stvarnoj implementaciji.