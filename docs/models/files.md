# Files servis – Modeli podataka

Svaka verzija fajla ima sopstveni objectKey, koji omogućava pronalaženje njenog sadržaja u MinIO skladištu.

Files servis koristi MongoDB za čuvanje metapodataka o fajlovima, folderima i njihovim verzijama, dok se stvarni sadržaj fajlova čuva u MinIO skladištu.

## Kolekcija `folders`

Sadrži informacije o folderima koje kreiraju korisnici.

| Atribut | Tip | Opis |
|---|---|---|
| `_id` | ObjectID | Jedinstveni identifikator foldera |
| `ownerId` | ObjectID | Identifikator vlasnika foldera |
| `name` | String | Naziv foldera |
| `parentId` | ObjectID / null | Identifikator roditeljskog foldera; null označava korenski folder |
| `createdAt` | DateTime | Vreme kreiranja |
| `updatedAt` | DateTime | Vreme poslednje izmene |

## Kolekcija `files`

Sadrži osnovne informacije o fajlovima. Sadržaj fajlova čuva se u MinIO skladištu.

| Atribut | Tip | Opis |
|---|---|---|
| `_id` | ObjectID | Jedinstveni identifikator fajla |
| `ownerId` | ObjectID | Identifikator vlasnika fajla |
| `name` | String | Naziv fajla |
| `parentId` | ObjectID / null | Identifikator foldera u kojem se fajl nalazi |
| `currentVersion` | Int | Broj trenutno važeće verzije |
| `createdAt` | DateTime | Vreme kreiranja |
| `updatedAt` | DateTime | Vreme poslednje izmene |

## Kolekcija `file_versions`

Sadrži podatke o pojedinačnim verzijama fajlova.

| Atribut | Tip | Opis |
|---|---|---|
| `_id` | ObjectID | Jedinstveni identifikator verzije |
| `fileId` | ObjectID | Identifikator fajla kojem verzija pripada |
| `versionNumber` | Int | Redni broj verzije |
| `objectKey` | String | Ključ za pronalaženje sadržaja verzije u MinIO skladištu |
| `sizeBytes` | Int64 | Veličina verzije u bajtovima |
| `contentType` | String | MIME tip sadržaja |
| `createdAt` | DateTime | Vreme kreiranja verzije |

## Povezanost podataka

- Svaki folder pripada jednom korisniku.
- Folder može da sadrži druge foldere i fajlove.
- Svaki fajl pripada jednom korisniku i može se nalaziti u određenom folderu.
- Svaki fajl može imati više verzija.
- Svaka verzija pripada jednom fajlu.
- `objectKey` povezuje zapis verzije sa stvarnim sadržajem u MinIO skladištu.
- Fajl može biti smešten samo u folder koji pripada istom korisniku.
- Folder može imati roditeljski folder samo ako roditeljski folder pripada istom korisniku.

## Predloženi indeksi

- `folders`: složeni indeks nad `ownerId` i `parentId`.
- `files`: složeni indeks nad `ownerId` i `parentId`.
- file_versions: jedinstveni složeni indeks nad fileId i versionNumber, koji sprečava postojanje dve verzije sa istim rednim brojem za isti fajl.

# HTTP API

Svi endpoint-i Files servisa zahtevaju autentifikaciju putem Bearer JWT tokena sa ulogom `user`.

## POST /folders – Kreiranje foldera

Kreira novi folder u root direktorijumu ili unutar postojećeg foldera.

**Zaglavlja:**
- `Authorization: Bearer <token>`
- `Content-Type: application/json`

**Telo zahteva:**

```json
{
  "name": "Dokumenti",
  "parentId": null
}
```

- `name` – obavezan naziv foldera.
- `parentId` – opcioni ID roditeljskog foldera. Vrednost `null` označava root direktorijum.

**HTTP odgovori:**
- `201 Created` – folder uspešno kreiran.
- `400 Bad Request` – neispravno telo zahteva, naziv ili parentId.
- `401 Unauthorized` – nedostaje ili nije validan token.
- `404 Not Found` – roditeljski folder ne postoji ili nije dostupan.
- `500 Internal Server Error` – greška pri kreiranju foldera.

## GET /folders – Prikaz foldera

Vraća listu foldera autentifikovanog korisnika.

**Zaglavlja:**
- `Authorization: Bearer <token>`

**Opcioni query parametar:**
- `parentId` – ID roditeljskog foldera. Ako nije naveden, vraćaju se folderi iz root direktorijuma.

**Primeri:**

```http
GET /folders
GET /folders?parentId=<folderId>
```

**HTTP odgovori:**
- `200 OK` – uspešno vraćena lista foldera.
- `400 Bad Request` – neispravan parentId.
- `401 Unauthorized` – nedostaje ili nije validan token.
- `404 Not Found` – roditeljski folder ne postoji ili nije dostupan.
- `500 Internal Server Error` – greška pri prikazu foldera.

## POST /files – Upload fajla

Otprema fajl u root direktorijum ili u postojeći folder. Sadržaj fajla čuva se u MinIO skladištu, dok se metapodaci i informacije o verziji čuvaju u MongoDB bazi.

**Zaglavlja:**
- `Authorization: Bearer <token>`
- `Content-Type: multipart/form-data`

**Polja zahteva:**

| Polje | Tip | Obavezno | Opis |
|---|---|---|---|
| `file` | File | Da | Fajl koji se otprema |
| `parentId` | String | Ne | ID postojećeg roditeljskog foldera |

**Ograničenja:**
- Dozvoljen je tačno jedan fajl po zahtevu.
- Maksimalna veličina fajla je 50 MiB.
- Naziv fajla se normalizuje radi uklanjanja putanja.
- MIME tip se automatski određuje na osnovu sadržaja fajla.
- Ako je naveden parentId, folder mora pripadati autentifikovanom korisniku.
- Ako parentId nije naveden ili je prazan, fajl se smešta u root direktorijum.

**HTTP odgovori:**
- `201 Created` – fajl uspešno otpremljen.
- `400 Bad Request` – neispravan multipart zahtev, naziv fajla, veličina ili parentId.
- `401 Unauthorized` – nedostaje ili nije validan token.
- `404 Not Found` – roditeljski folder ne postoji ili nije dostupan.
- `413 Request Entity Too Large` – fajl ili HTTP zahtev je prevelik.
- `500 Internal Server Error` – greška pri upload-u fajla.