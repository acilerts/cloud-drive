# Sharing servis – Model podataka

Sharing servis koristi Neo4j graf bazu podataka za upravljanje grupama korisnika i pravima pristupa fajlovima i folderima.

Model je zasnovan na čvorovima i vezama. Korisnici, fajlovi i folderi predstavljeni su referencama na identifikatore koje generišu Users i Files servisi.

## Čvorovi

### `User`

Predstavlja korisnika koji učestvuje u deljenju sadržaja.

| Atribut | Tip | Opis |
|---|---|---|
| `userId` | String | Identifikator korisnika iz Users servisa |

### `Group`

Predstavlja grupu korisnika kojoj se mogu dodeliti prava pristupa sadržaju.

| Atribut | Tip | Opis |
|---|---|---|
| `groupId` | String | Jedinstveni identifikator grupe |
| `name` | String | Naziv grupe |
| `ownerId` | String | Identifikator korisnika koji je kreirao grupu |
| `createdAt` | DateTime | Vreme kreiranja grupe |

### `File`

Predstavlja referencu na fajl kojim upravlja Files servis.

| Atribut | Tip | Opis |
|---|---|---|
| `fileId` | String | Identifikator fajla iz Files servisa |

### `Folder`

Predstavlja referencu na folder kojim upravlja Files servis.

| Atribut | Tip | Opis |
|---|---|---|
| `folderId` | String | Identifikator foldera iz Files servisa |

## Veze između čvorova

| Veza | Izvor | Odredište | Opis |
|---|---|---|---|
| `OWNS` | User | Group | Korisnik je vlasnik grupe |
| `MEMBER_OF` | User | Group | Korisnik je član grupe |
| `OWNS` | User | File | Korisnik je vlasnik fajla |
| `OWNS` | User | Folder | Korisnik je vlasnik foldera |
| `HAS_ACCESS` | User | File | Korisnik ima dodeljen pristup fajlu |
| `HAS_ACCESS` | User | Folder | Korisnik ima dodeljen pristup folderu |
| `HAS_ACCESS` | Group | File | Grupa ima dodeljen pristup fajlu |
| `HAS_ACCESS` | Group | Folder | Grupa ima dodeljen pristup folderu |

## Atributi veza

Veza `HAS_ACCESS` može imati sledeći atribut:

| Atribut | Tip | Opis |
|---|---|---|
| `permission` | String | Nivo pristupa, na primer `read` ili `write` |

## Povezanost sa drugim servisima

- Users servis predstavlja izvor identifikatora korisnika.
- Files servis predstavlja izvor identifikatora fajlova i foldera.
- Sharing servis čuva grupe i veze između korisnika i sadržaja.
- Prilikom brisanja ili izmene sadržaja, Sharing servis može dobiti odgovarajući događaj od Files servisa i ažurirati svoje podatke.

**Napomena:** Ovo je predloženi model podataka. Konačni atributi i pravila pristupa treba da budu usaglašeni sa specifikacijom projekta i članovima tima pre implementacije.