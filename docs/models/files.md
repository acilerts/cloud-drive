# Files servis – Modeli podataka

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

## Predloženi indeksi

- `folders`: složeni indeks nad `ownerId` i `parentId`.
- `files`: složeni indeks nad `ownerId` i `parentId`.
- `file_versions`: složeni indeks nad `fileId` i `versionNumber`.