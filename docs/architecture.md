# Arhitektura sistema

Cloud Drive je platforma za skladištenje, organizaciju i deljenje fajlova. Sistem je organizovan kao skup nezavisnih servisa, pri čemu svaki servis ima svoju odgovornost i odgovarajuće skladište podataka.

## Komponente sistema

| Komponenta | Odgovornost | Port |
|---|---|---:|
| API Gateway | Jedinstvena ulazna tačka za klijentske zahteve | 8080 |
| Users | Korisnički nalozi i autentifikacija | 8081 |
| Files | Upravljanje fajlovima i njihovim verzijama | 8082 |
| Sharing | Upravljanje deljenjem fajlova i foldera | 8083 |
| Notifications | Čuvanje i dostavljanje obaveštenja | 8084 |
| Analytics | Prikupljanje i pregled analitičkih podataka | 8085 |

## Skladištenje podataka

Za različite potrebe sistema predviđena su sledeća skladišta:

| Komponenta | Tehnologija |
|---|---|
| Users | MongoDB |
| Files | MongoDB i MinIO |
| Sharing | Neo4j |
| Notifications | Cassandra |
| Analytics | MongoDB |

API Gateway ne poseduje sopstvenu bazu podataka.

## Komunikacija između servisa

Klijentska aplikacija pristupa sistemu preko API Gateway-a. Gateway prosleđuje zahteve odgovarajućim servisima, tako da klijent ne mora direktno da komunicira sa svakim servisom.

Servisi međusobno razmenjuju podatke sinhrono, putem HTTP zahteva, ili asinhrono, putem događaja, u zavisnosti od potrebe konkretne operacije.

Jedan primer sinhrone komunikacije je provera korisničke kvote: prilikom otpremanja fajla, Files servis proverava kod Users servisa koliko prostora korisnik može da koristi.

## Konfiguracija

Portovi i ostale vrednosti konfiguracije učitavaju se iz eksternih konfiguracionih fajlova i promenljivih okruženja. Vrednosti konfiguracije ne treba direktno upisivati u izvorni kod.

