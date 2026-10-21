# Design Doc: ssh2rclone

**Data:** 2026-10-20  
**Stato:** Approvato  
**Autori:** Michele, Antigravity  

---

## 1. Obiettivo e Visione

`ssh2rclone` è uno strumento da riga di comando scritto in **Go** per convertire in modo idempotente le configurazioni host presenti in `~/.ssh/config` in definizioni di remote SFTP all'interno di `rclone.conf`.

### Requisiti Chiave
1. **Idempotenza e Delimitatori Dedicati:** Il codice generato viene racchiuso tra tag delimitatori (`# --- BEGIN ssh2rclone GENERATED BLOCK ---` e `# --- END ssh2rclone GENERATED BLOCK ---`). Rilanciando l'eseguibile, il blocco viene interamente rigenerato senza mai toccare le sezioni esterne definite manualmente dall'utente.
2. **Isolamento Rigoroso e Gestione Conflitti:** Nessuna riga al di fuori dei tag delimitatori viene modificata. Se un host SSH ha lo stesso nome di una sezione manuale preesistente esterna, viene **saltato con un warning** esplicito.
3. **Modalità Semplice vs OpenSSH Nativo:**
   * **Host Semplici:** Generazione parametri nativi SFTP di rclone (`type = sftp`, `host`, `user`, `port`, `key_file`).
   * **Host Complessi (`ProxyJump`, `ProxyCommand`, o forzati via `--all-ssh`):** Delega a OpenSSH locale (`type = sftp`, `ssh = ssh <alias>`), consentendo a rclone di ereditare jump host e configurazioni avanzate.
4. **Risoluzione Automatica dei Percorsi (incluso XDG):** Rilevamento dinamico di `rclone.conf` tramite cascata: flag CLI > env `$RCLONE_CONFIG` > interrogazione di `rclone config file` > specifica XDG (`$XDG_CONFIG_HOME/rclone/rclone.conf` o `~/.config/rclone/rclone.conf`).
5. **Sicurezza Operativa:** Supporto a `--dry-run`, creazione di backup preventivo (`rclone.conf.bak`), e scrittura atomica tramite file temporaneo e rename.

---

## 2. Architettura del Progetto

Il codice risiede in un modulo Go (`ssh2rclone`) strutturato per layer ben definiti all'interno di `internal/`:

```text
ssh2rclone/
├── main.go                     # Entrypoint CLI, parsing flag, orchestrazione
├── internal/
│   ├── model/                  # Struct pure dei dati (SSHEntry, RcloneRemote, RunConfig)
│   ├── helper/                 # Utility generiche (scrittura atomica, backup, manipolazione stringhe)
│   ├── config/                 # Risoluzione percorsi (SSH config, rclone.conf XDG/CLI/rclone tool)
│   └── services/               # Business logic
│       ├── ssh_parser.go       # Parsing ~/.ssh/config, ereditarietà wildcard
│       └── rclone_sync.go      # Conversione modelli, gestione delimitatori e merge rclone.conf
├── go.mod
├── go.sum
└── README.md
```

### Dipendenze Esterne
* `github.com/kevinburke/ssh_config`: libreria consolidata per il parsing conforme dello standard OpenSSH config (supporta `Include`, wildcard, quoting, case-insensitivity).
* Go Standard Library (`os`, `path/filepath`, `bufio`, `strings`, `flag`, `os/exec`).

---

## 3. Modelli Dati (`internal/model`)

### `SSHEntry`
Rappresenta un singolo host estratto da `ssh_config`:
* `Alias` (`string`): nome dell'host/alias (es. `myserver`).
* `HostName` (`string`): hostname o IP di destinazione (se non specificato, coincide con `Alias`).
* `User` (`string`): utente SSH.
* `Port` (`string`): porta SSH (default `22` se non specificata).
* `IdentityFile` (`string`): percorso della chiave privata SSH (con espansione `~`).
* `ProxyJump` (`string`): host o catena di jump host.
* `ProxyCommand` (`string`): comando proxy arbitrario.
* `IsComplex` (`bool`): `true` se `ProxyJump != ""` o `ProxyCommand != ""`.

### `RcloneRemote`
Rappresenta una sezione di remote da inserire in `rclone.conf`:
* `Name` (`string`): nome del blocco `[nome]`.
* `Type` (`string`): fisso `"sftp"`.
* `Host` (`string`): parametro `host` (se semplice).
* `User` (`string`): parametro `user` (se semplice).
* `Port` (`string`): parametro `port` (se semplice).
* `KeyFile` (`string`): parametro `key_file` (se semplice e valorizzato).
* `SSHCommand` (`string`): parametro `ssh` (es. `ssh myserver` se complesso).

### `SyncOptions`
Opzioni passate a runtime:
* `SSHConfigPath` (`string`): percorso del file SSH da leggere.
* `RcloneConfigPath` (`string`): percorso del file `rclone.conf` da aggiornare.
* `DryRun` (`bool`): stampa a video le modifiche senza scrivere su disco.
* `AllSSH` (`bool`): forza tutti gli host in modalità OpenSSH delegation.
* `NoBackup` (`bool`): disabilita il backup `.bak`.
* `Verbose` (`bool`): output dettagliato di debug.

---

## 4. Flusso Dati e Componenti

### 4.1 Risoluzione Percorsi (`internal/config`)
1. **SSH Config:**
   * Se `--ssh-config` è fornito, usa quel path.
   * Altrimenti: `~/.ssh/config` (tramite `os.UserHomeDir`).
2. **Rclone Config:**
   * Livello 1: flag `--rclone-config`.
   * Livello 2: variabile d'ambiente `$RCLONE_CONFIG`.
   * Livello 3: se il comando `rclone` è disponibile in `$PATH`, esegue `rclone config file` ed estrae il percorso indicato.
   * Livello 4 (XDG Fallback):
     * Se `$XDG_CONFIG_HOME` è impostato: `$XDG_CONFIG_HOME/rclone/rclone.conf`.
     * Altrimenti: `~/.config/rclone/rclone.conf`.
     * Se non esiste ma è presente `~/.rclone.conf` (legacy), usa quello.

### 4.2 Parsing SSH ed Ereditarietà (`internal/services/ssh_parser.go`)
1. Lettura del file SSH con gestione automatica di eventuali `Include`.
2. Itera tutte le dichiarazioni `Host`:
   * Se il pattern contiene wildcard (`*`, `?`), **non** genera un remote, ma i suoi valori (`User`, `Port`, `IdentityFile`) fungono da default per gli host concreti.
   * Se un blocco contiene più alias (es. `Host server1 server2`), ciascun alias viene processato come entry separata.
3. Per ciascun host concreto:
   * Recupera i parametri risolti (`HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`, `ProxyCommand`).
   * Valuta se l'host è complesso (`ProxyJump != ""` o `ProxyCommand != ""` o `AllSSH == true`).

### 4.3 Gestione `rclone.conf` (`internal/services/rclone_sync.go`)
1. **Analisi file esistente:**
   * Se `rclone.conf` non esiste, crea la struttura iniziale con il blocco generato.
   * Se esiste, legge il contenuto e individua:
     * `Prefix`: contenuto prima di `# --- BEGIN ssh2rclone GENERATED BLOCK ---`.
     * `Suffix`: contenuto dopo `# --- END ssh2rclone GENERATED BLOCK ---`.
   * Estrae la lista dei nomi di sezione `[remote]` presenti in `Prefix` e `Suffix` (sezioni manuali esterne).
2. **Rilevamento Conflitti:**
   * Per ogni `SSHEntry`: se il nome `[alias]` esiste già nelle sezioni manuali esterne, viene scartato emettendo un warning:
     `[WARN] Remote '<alias>' già esistente nella configurazione manuale di rclone.conf: ignorato per sicurezza.`
3. **Generazione Blocco:**
   * Genera il testo INI per tutti gli host ammessi all'interno dei tag delimitatori.
4. **Applicazione:**
   * Se `DryRun` è attivo: stampa il blocco generato su standard output ed esce con codice 0.
   * Se `DryRun` è disattivato:
     * Se `NoBackup` è falso ed il file originale esiste, copia `rclone.conf` in `rclone.conf.bak`.
     * Scrive il nuovo contenuto completo (`Prefix` + `Nuovo Blocco` + `Suffix`) in un file temporaneo `rclone.conf.tmp.*` nella stessa cartella.
     * Esegue `os.Rename` atomico sul file di destinazione.

---

## 5. Interfaccia a Riga di Comando (CLI)

```bash
# Esecuzione standard automatica (aggiorna rclone.conf e crea backup)
ssh2rclone

# Anteprima a video senza modificare file
ssh2rclone --dry-run

# Forza tutti gli host a usare la delega openssh (ssh = ssh <alias>)
ssh2rclone --all-ssh

# Percorsi personalizzati
ssh2rclone --ssh-config /path/to/config --rclone-config /path/to/rclone.conf

# Disattiva backup automatico
ssh2rclone --no-backup

# Output verboso per diagnostica
ssh2rclone -v
```

---

## 6. Sicurezza e Casi Limite

| Caso Limite | Comportamento Atteso |
|---|---|
| `~/.ssh/config` inesistente | Errore chiaro a video, exit code 1, nessun file modificato. |
| `rclone.conf` inesistente | Crea la directory genitore (es. `~/.config/rclone`) e scrive il file contenente solo il blocco generato. |
| Sezioni esterne con stesso nome di un host | L'host SSH viene saltato con `[WARN]`, preservando integralmente il remote esterno. |
| Blocco già presente da precedente esecuzione | Il vecchio blocco viene interamente rimpiazzato dal nuovo (idempotenza). |
| Crash improvviso o interruzione di processo | Grazie alla scrittura su file `.tmp` e successivo rename atomico, `rclone.conf` non può mai trovarsi troncato o corrotto. |
| Host con chiavi con percorso `~` | Il percorso viene espanso con la reale home directory dell'utente prima della scrittura. |

---

## 7. Strategia di Test

1. **Unit Test `internal/services/ssh_parser_test.go`:**
   * Test con file SSH contenente:
     * Host semplice con `HostName`, `User`, `Port`, `IdentityFile`.
     * Host con `ProxyJump` e `ProxyCommand`.
     * Direttiva globale `Host *` con parametri ereditati.
     * Host multipli sulla stessa riga (`Host alias1 alias2`).
2. **Unit Test `internal/services/rclone_sync_test.go`:**
   * Aggiornamento idempotente: eseguire due sync consecutive produce lo stesso output.
   * Preservazione sezioni esterne: remote definiti prima o dopo il blocco restano intatti.
   * Conflitto nomi: se esiste un remote esterno, viene skippato.
3. **Integration Test:**
   * Test end-to-end con filesystem temporaneo (`t.TempDir()`), verifica flag `--dry-run`, verifica creazione backup `.bak` e sostituzione atomica.
