# Način rada (Git)

## Osnovna pravila

- `main` je glavna grana i u njoj se nalazi stabilna verzija projekta.
- Nije dozvoljen direktan push na `main`. Sve izmene se unose preko Pull Request-a (PR).
- Nije dozvoljeno brisanje ili force push na `main`.
- Svaki član radi na svojoj grani. Nije dozvoljeno zajedničko menjanje iste grane.
- Svaki PR mora da pregleda i odobri najmanje jedan drugi član tima.
- Autor PR-a ne može sam sebi da odobri izmene.
- PR se ne spaja dok nisu rešeni svi komentari i konflikti.
- Izmene koje utiču na više servisa ili zajedničku arhitekturu moraju se prethodno dogovoriti sa ostalim članovima.
- Članovi ne menjaju samostalno fajlove u folderima servisa za koje nisu zaduženi, osim uz prethodni dogovor.

## Imenovanje grana

Grane se imenuju prema servisu ili oblasti na kojoj se radi:

`<servis>/<opis-izmene>`

Primeri:

- `users/otp-login`
- `files/upload`
- `gateway/rate-limit`
- `docs/contracts`

Za ispravke grešaka može se koristiti prefiks `fix/`, a za zajedničke izmene `shared/`.

## Commit poruke

Commit poruke treba da imaju prefiks servisa ili oblasti, a zatim kratak opis izmene.

Primeri:

- `users: add otp verification`
- `files: fix upload validation`
- `gateway: add rate limit`
- `docs: update api contracts`

Jedan commit treba da predstavlja jednu logičku celinu. Commit poruke treba da budu kratke i jasne.

## Tok rada

```bash
# 1. Preuzeti najnovije izmene sa main grane
git switch main
git pull origin main

# 2. Napraviti novu granu
git switch -c users/otp-login

# 3. Raditi na izmeni i napraviti commit
git add services/users
git commit -m "users: add otp verification"

# 4. Proveriti najnovije izmene na GitHub-u
git fetch origin

# 5. Ažurirati svoju granu u odnosu na main
git rebase origin/main

# 6. Poslati granu na GitHub
git push -u origin users/otp-login
```

Nakon slanja grane, na GitHub-u se otvara Pull Request prema grani `main`.

PR treba da sadrži:
- kratak opis urađene izmene;
- spisak važnijih promenjenih delova;
- informacije o testiranju;
- napomenu ako izmena utiče na druge servise ili zajedničke komponente.

Drugi član tima pregleda izmene. Ako postoje problemi, autor ih ispravlja na svojoj grani i ponovo šalje izmene.

Spajanje se vrši tek nakon odobrenja PR-a i uspešnih provera.

## Pravila za rebase

- Rebase se radi samo na sopstvenoj grani, nikada na `main`.
- Pre rebase-a potrebno je preuzeti najnovije izmene pomoću `git fetch origin`.
- Ako rebase prijavi konflikt, potrebno je otvoriti označeni fajl i ručno rešiti konflikt.
- Nakon rešavanja konflikta koristi se:

```bash
git add <fajl>
git rebase --continue
```

- Ako nije moguće rešiti konflikt, postupak se može prekinuti komandom:

```bash
git rebase --abort
```

- Force push se ne koristi kao uobičajen deo rada. Ako je nakon rebase-a potrebno ažurirati već poslatu granu, koristi se `--force-with-lease`, i to samo na sopstvenoj grani.

## Provera pre spajanja

Pre odobravanja PR-a potrebno je proveriti:

- da li se projekat uspešno kompajlira;
- da li postojeći testovi prolaze;
- da li izmena narušava postojeću funkcionalnost;
- da li su ažurirani API ugovori i dokumentacija ako je to potrebno;
- da li se Docker Compose konfiguracija i servisi pokreću, kada je izmena relevantna za njih.

Provere se vremenom mogu automatizovati pomoću GitHub Actions.

## Tagovi

Na kraju svake značajne faze razvoja pravi se Git tag koji označava stanje projekta.

Primeri:

- `v6-checkpoint`
- `v7`
- `v8`
- `v9`
- `v10`
- `v10-plus`

Tagove postavlja član tima koji je zadužen za održavanje glavne grane, nakon dogovora sa ostalim članovima.