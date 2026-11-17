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

Klijentska aplikacija pristupa sistemu preko API Gateway-a. Gateway predstavlja jedinstvenu ulaznu tačku sistema i prosleđuje zahteve odgovarajućim servisima, tako da klijent ne mora direktno da komunicira sa svakim servisom.

Servisi međusobno razmenjuju podatke na dva načina:

- **Sinhrona komunikacija** – putem HTTP zahteva i odgovora, kada je servisu potreban neposredan odgovor drugog servisa kako bi nastavio obradu zahteva.
- **Asinhrona komunikacija** – putem događaja, kada servis objavljuje informaciju o nekoj aktivnosti, a drugi servisi mogu da je obrade nezavisno od servisa koji je događaj proizveo.

### Sinhrona komunikacija

Sinhrona komunikacija koristi se kada je za izvršavanje operacije neophodan neposredan odgovor drugog servisa.

Primer: prilikom otpremanja fajla, Files servis proverava kod Users servisa podatke o korisničkoj kvoti. Users servis vraća informaciju o raspoloživom prostoru, nakon čega Files servis može da nastavi ili odbije otpremanje fajla.

### Asinhrona komunikacija

Asinhrona komunikacija koristi se za razmenu događaja između servisa, kada nije potrebno da servis koji je proizveo događaj čeka da ga drugi servisi obrade.

Primeri događaja:

- Sharing servis može da proizvede događaj kada korisnik dobije pristup fajlu ili fascikli.
- Files servis može da proizvede događaj kada se otpremi nova verzija fajla.
- Notifications servis može da obradi odgovarajući događaj i kreira obaveštenje za korisnika.
- Analytics servis može da obradi događaje i sačuva podatke o aktivnostima korisnika radi kasnije analize.

Na ovaj način servisi mogu da reaguju na aktivnosti drugih servisa bez direktne međusobne zavisnosti tokom obrade događaja.

## Konfiguracija

Portovi i ostale vrednosti konfiguracije učitavaju se iz eksternih konfiguracionih fajlova i promenljivih okruženja. Vrednosti konfiguracije ne treba direktno upisivati u izvorni kod.

