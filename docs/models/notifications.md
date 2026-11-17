# Notifications servis – Modeli podataka

Notifications servis koristi Cassandra bazu podataka za čuvanje obaveštenja namenjenih korisnicima sistema.

Model podataka organizovan je tako da omogućava pronalaženje obaveštenja određenog korisnika, prikaz najnovijih obaveštenja i evidenciju pročitanih obaveštenja.

## Tabela `notifications`

Tabela `notifications` čuva obaveštenja koja korisnici dobijaju tokom korišćenja sistema.

| Atribut | Tip | Opis |
|---|---|---|
| `user_id` | UUID | Identifikator korisnika kome je obaveštenje namenjeno |
| `notification_id` | UUID | Jedinstveni identifikator obaveštenja |
| `type` | TEXT | Vrsta obaveštenja |
| `title` | TEXT | Naslov obaveštenja |
| `message` | TEXT | Tekst obaveštenja |
| `is_read` | BOOLEAN | Označava da li je obaveštenje pročitano |
| `created_at` | TIMESTAMP | Vreme kreiranja obaveštenja |
| `related_resource_id` | UUID | Identifikator povezanog resursa, na primer fajla |

## Primarni ključ

Predloženi primarni ključ:

`PRIMARY KEY ((user_id), created_at, notification_id)`

- `user_id` je partition key i određuje kojoj particiji pripadaju obaveštenja.
- `created_at` je clustering column i omogućava sortiranje obaveštenja po vremenu nastanka.
- `notification_id` je dodatna clustering column koja obezbeđuje jedinstveno razlikovanje obaveštenja nastalih u istom trenutku.

Podaci se organizuju prema korisniku, tako da se obaveštenja jednog korisnika mogu efikasno pronaći.

## Vrste obaveštenja

Predviđene vrste obaveštenja obuhvataju:

- `sharing` – obaveštenje o deljenju fajla ili foldera;
- `new_version` – obaveštenje o kreiranju nove verzije fajla;
- `system` – sistemsko obaveštenje.

## Povezanost sa drugim servisima

Notifications servis može da dobija informacije od drugih servisa putem događaja.

Primeri:
- Sharing servis šalje događaj kada korisnik dobije pristup fajlu ili folderu.
- Files servis šalje događaj kada se kreira nova verzija fajla.

Na osnovu primljenih događaja Notifications servis kreira i čuva odgovarajuća obaveštenja.

Ovaj model predstavlja predlog za arhitekturu sistema i potrebno ga je usaglasiti sa timom pre implementacije.