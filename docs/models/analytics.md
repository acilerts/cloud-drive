# Analytics servis – Modeli podataka

Analytics servis koristi MongoDB bazu podataka za čuvanje podataka o aktivnostima korisnika i generisanje različitih statistika o korišćenju sistema.

Servis prima podatke o događajima koji nastaju u drugim servisima, čuva ih i omogućava njihovu kasniju analizu.

## Kolekcija `activity_events`

Kolekcija `activity_events` čuva pojedinačne događaje koji predstavljaju aktivnosti korisnika u sistemu.

| Polje | Tip | Opis |
|---|---|---|
| `_id` | ObjectId | Jedinstveni identifikator dokumenta |
| `eventId` | String | Jedinstveni identifikator događaja |
| `userId` | String | Identifikator korisnika koji je izvršio aktivnost |
| `eventType` | String | Vrsta događaja |
| `resourceType` | String | Vrsta resursa na koji se događaj odnosi |
| `resourceId` | String | Identifikator povezanog resursa |
| `metadata` | Object | Dodatni podaci o događaju |
| `occurredAt` | Date | Vreme kada se aktivnost dogodila |
| `receivedAt` | Date | Vreme kada je Analytics servis primio događaj |

### Opis polja

- `eventId` – omogućava jedinstvenu identifikaciju događaja i sprečavanje višestrukog evidentiranja istog događaja.
- `userId` – identifikuje korisnika koji je izvršio aktivnost.
- `eventType` – označava vrstu aktivnosti, na primer `file_uploaded`, `file_deleted`, `folder_created` ili `file_shared`.
- `resourceType` – označava tip resursa, na primer `file` ili `folder`.
- `resourceId` – identifikuje konkretan resurs na koji se aktivnost odnosi.
- `metadata` – sadrži dodatne informacije koje zavise od vrste događaja.
- `occurredAt` – čuva vreme nastanka događaja.
- `receivedAt` – čuva vreme prijema događaja u Analytics servisu.

## Indeksi

Predloženi indeksi:

- Jedinstveni indeks nad poljem `eventId` radi sprečavanja dupliranja događaja.
- Indeks nad kombinacijom `userId` i `occurredAt` radi efikasnog pronalaženja aktivnosti određenog korisnika.
- Indeks nad kombinacijom `eventType` i `occurredAt` radi analize učestalosti različitih vrsta aktivnosti.

## Generisanje statistika

Statistike se mogu izračunavati nad kolekcijom `activity_events` korišćenjem MongoDB agregacionih operacija.

Primeri mogućih statistika:

- Broj aktivnosti po korisniku.
- Broj otpremljenih i obrisanih datoteka.
- Broj kreiranih fascikli.
- Učestalost različitih vrsta aktivnosti.
- Aktivnosti korisnika u određenom vremenskom periodu.

## Napomena

Ovaj model predstavlja predlog za organizaciju podataka Analytics servisa. Nije implementiran i potrebno ga je usaglasiti sa ostatkom tima.

Tačan format događaja i način njihove razmene između servisa biće definisani kroz dokumentaciju komunikacije i ugovora o događajima.