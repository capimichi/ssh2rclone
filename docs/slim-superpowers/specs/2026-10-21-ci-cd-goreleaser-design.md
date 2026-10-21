# Design Doc: CI/CD & GoReleaser Automation for ssh2rclone

**Date:** 2026-10-21  
**Status:** Approved  
**Author:** Pair programming session (Michele & Antigravity)

---

## 1. Obiettivo e Contesto

`ssh2rclone` è una CLI scritta in Go che converte le configurazioni `~/.ssh/config` in remote SFTP idempotenti per `rclone.conf`.
L'obiettivo di questo documento è definire l'infrastruttura di Continuous Integration e Continuous Delivery (CI/CD) per automatizzare la verifica del codice, la compilazione multi-piattaforma, il versioning e il rilascio binario su GitHub via GoReleaser.

---

## 2. Architettura e Componenti

### 2.1 Gestione Versioning e Flag `--version`

* **Variabili d'ambiente compilate via Ldflags:**
  In `main.go`, definiamo variabili package-level con valori di default per lo sviluppo locale:
  ```go
  var (
      version = "dev"
      commit  = "none"
      date    = "unknown"
  )
  ```
* **Flag CLI `--version` (e `-version`):**
  * Registrato tramite standard library `flag`.
  * Se invocato, stampa: `ssh2rclone version <version> (commit: <commit>, date: <date>)`.
  * Termina immediatamente con exit code `0` prima di verificare la presenza di file di configurazione (`~/.ssh/config`, `rclone.conf`).
* **Test:**
  * Test in `main_test.go` per validare il comportamento del flag di versione.

### 2.2 GoReleaser (`.goreleaser.yaml`)

Configurazione GoReleaser v2:
* **Builds:**
  * Target binary: `ssh2rclone`
  * Main package: `./main.go`
  * Sistemi operativi (`goos`): `linux`, `darwin`, `windows`
  * Architetture (`goarch`): `amd64`, `arm64`
  * Flags: `-trimpath`
  * Ldflags: `-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}`
* **Archivi:**
  * Formato `.tar.gz` per Unix (`linux`, `darwin`).
  * Formato `.zip` per `windows`.
  * File inclusi nell'archivio: `README.md`, `LICENSE` (se presente).
* **Checksum:**
  * Algoritmo: SHA-256 (`ssh2rclone_{{ .Version }}_checksums.txt`).
* **Changelog:**
  * Raggruppamento automatico basato sui commit conventional (`feat`, `fix`, `docs`, `refactor`, ecc.).

### 2.3 GitHub Actions Workflows

Due workflow distinti sotto `.github/workflows/`:

#### A. CI Pipeline (`.github/workflows/ci.yml`)
* **Trigger:**
  * `push` sui rami `main`
  * `pull_request` verso `main`
* **Permessi:** `contents: read`
* **Job `test`:**
  * Runner: `ubuntu-latest`
  * Go setup: versione estratta da `go.mod` con cache abilitata.
  * Comando: `go test -v -race ./...`
* **Job `lint`:**
  * Runner: `ubuntu-latest`
  * Step: `golangci/golangci-lint-action@v6` con configurazione standard Go.

#### B. Release Pipeline (`.github/workflows/release.yml`)
* **Trigger:**
  * `push` su tag `tags: ['v*']` (es. `v0.1.0`)
* **Permessi:** `contents: write`
* **Job `release`:**
  * Runner: `ubuntu-latest`
  * Step 1: Checkout con `actions/checkout@v4` e `fetch-depth: 0` (necessario per estrarre la cronologia dei tag per il changelog e i metadati git).
  * Step 2: Setup Go (`actions/setup-go@v5`).
  * Step 3: GoReleaser Action (`goreleaser/goreleaser-action@v6`) con argomenti `release --clean` e ambiente `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`.

### 2.4 Developer Tooling: `Makefile`

Un `Makefile` alla radice del repository per standardizzare i workflow locali:
* `make build`: compila `./bin/ssh2rclone` con variabili ldflags calcolate localmente via `git`.
* `make test`: esegue `go test -v -race ./...`.
* `make lint`: esegue `golangci-lint run`.
* `make snapshot`: esegue `goreleaser release --snapshot --clean` per validare la packaging locale.
* `make clean`: rimuove `./bin/` e `dist/`.

### 2.5 Documentazione nel `README.md` e in `AGENTS.md`

1. **`README.md` (per utenti e collaboratori):**
   * Sezione dedicata a *"Release & Versioning"*:
     * Convenzione SemVer dei tag (`vMAJOR.MINOR.PATCH`, es. `v0.1.0`).
     * Come creare un tag annotato:
       ```bash
       git tag -a v0.1.0 -m "Release v0.1.0"
       git push origin v0.1.0
       ```
     * Spiegazione dell'automazione GitHub Actions e rilascio degli asset.

2. **`AGENTS.md` (per agenti AI e futuri sviluppi automatizzati):**
   * Creazione/aggiornamento del file `AGENTS.md` alla radice del repository:
     * Regole di tagging automatico e convenzione SemVer.
     * Istruzioni esplicite per gli agenti su quando e come eseguire bump di versione e push del tag per innescare la pipeline di release.
     * Checklist pre-tagging (es. `make test` e `make lint` devono passare prima di taggare).

---

## 3. Gestione Errori e Casi Limite

1. **Tag git non conforme o senza prefisso `v`:** il workflow di release viene innescato unicamente per pattern `v*`, evitando release accidentali su tag arbitrari.
2. **Commit non pulito o snapshot locale:** il flag `--snapshot` nel `Makefile` garantisce che lo sviluppatore possa testare la release senza sporcare il repository remoto o richiedere un token GitHub.
3. **Mancanza di file di config durante `ssh2rclone --version`:** il parser CLI intercetta il flag `--version` prima delle funzioni di risoluzione dei path, garantendo output immediato e codice d'uscita 0.
