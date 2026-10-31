# 0001: Jedan repozitorijum, jedan Compose, grane po funkcionalnosti

- **Status:** Prihvaćeno
- **Datum:** 2026-10-31

## Kontekst

Projekat se sastoji od više nezavisnih servisa koji zajedno čine jednu platformu. Potrebno je odabrati način organizacije izvornog koda i upravljanja servisima tokom razvoja.

Pošto projekat razvija mali tim, važno je da organizacija bude jednostavna za održavanje, zajednički rad i demonstraciju sistema.

## Odluka

- Ceo projekat se nalazi u jednom Git repozitorijumu (*monorepo*).
- Koristi se jedan Go modul: `github.com/acilerts/cloud-drive`.
- Svaki servis ima svoj folder unutar `services/`, na primer `services/users/` i `services/files/`.
- Jedan `compose.yaml` fajl u korenu projekta služi za pokretanje celog sistema.
- Razvoj se odvija kroz zasebne grane i Pull Request-ove, u skladu sa pravilima definisanim u `workflow.md`.

## Razlozi za odluku

- Jedan repozitorijum olakšava zajednički razvoj i pregled celog sistema.
- Jedan Compose fajl omogućava pokretanje svih komponenti sistema pomoću jedne komande.
- Organizacija servisa po folderima daje jasnu strukturu projekta i olakšava podelu posla.
- Jedan Go modul pojednostavljuje upravljanje zavisnostima na nivou projekta.
- Grane i Pull Request-ovi omogućavaju pregled izmena pre njihovog spajanja u glavnu granu.

## Razmotrene alternative

**Odvojeni Git repozitorijumi za svaki servis**

Ova opcija bi zahtevala zasebno podešavanje i održavanje svakog repozitorijuma. Za projekat koji razvija mali tim, dodatna organizacija ne donosi dovoljno koristi u odnosu na jedan zajednički repozitorijum.

**Više Compose fajlova, po jedan za svaki servis**

Ovakva organizacija bi omogućila odvojeno pokretanje servisa, ali bi zahtevala dodatno upravljanje njihovim konfiguracijama. Za potrebe ovog projekta, jedan Compose fajl u korenu predstavlja jednostavnije rešenje.

## Posledice odluke

- Promene na više servisa mogu se pratiti i pregledati u okviru istog Pull Request-a.
- Članovi tima moraju voditi računa o izmenama u zajedničkim fajlovima, posebno u `compose.yaml` i Go modulu.
- Konflikti mogu nastati kada više članova istovremeno menja zajedničke fajlove.
- Promene koje utiču na više servisa zahtevaju dogovor između članova tima.