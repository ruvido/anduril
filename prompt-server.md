
Anduril – File‑server di foto basato su filesystem + PocketBase

**Descrizione generale:**
Anduril è un unico eseguibile Go che fornisce:

1. **Server HTTP** con embedded PocketBase per:

    anduril server

   * Autenticazione multi‑utente
   * Gestione di album e preferiti
   * Funzioni CRUD via API e interfaccia web

2. **Watcher in tempo reale** sul filesystem per:

   * Rilevare automaticamente nuove immagini (JPG, PNG, HEIC…) aggiunte nelle cartelle `YYYY/MM/DD/` presenti in LIBRARY (path letto da config)
   * Upsert (creazione/aggiornamento) o cancellazione del record `photos` in PocketBase

3. **Comando “import”** one‑shot per eseguire un full‑sync manuale di tutta la libreria.

   * Estrarre data di scatto (dal path o da EXIF), calcolare hash per deduplica
   * ordinare i file in LIBRARY

    anduril import [folder]

---

### Specifica del prompt

> **Progetto Anduril**
>
> * **Linguaggio:** Go
>
> * **Dipendenze principali:**
>
>   * `github.com/pocketbase/pocketbase` (backend embedded + UI)
>   * `github.com/fsnotify/fsnotify` (watcher filesystem)
>   * `github.com/spf13/cobra` (CLI con sub‑comandi)
>   * `github.com/spf13/viper` (config TOML)
>
> * **File di configurazione (`config.toml`):**
>
>   ```toml
>   library = "/percorso/alla/tua/libreria/foto"
>   ```
>
> * **Structure CLI e comandi:**
>
>   ```bash
>   anduril server   # Avvia PocketBase + watcher (serve API + Web UI + static files)
>   anduril import [folder]   # Copia tutti i file media in [folder] in LIBRARY con deduplica (skip file uguali + tiene la foto della miglior qualita)
>   ```
>
> * **Comportamento `server`:**
>
>   1. Legge `library` da config
>   2. Avvia PocketBase embedded (porta di default 8080)
>   4. Avvia in background il watcher FS che, per ogni evento `Create`, `Remove`, `Rename`, chiama rispettivamente `handleUpsert(path)` o `handleDelete(path)`
>   5. PocketBase serve le immagini statiche da `library` su `/static/…`, e l’UI/API per album/utenti.
>
> * **Comportamento `import`:**
>
>   1. Legge `library` da config
>   2. Copia immagine da [folder] a `library` usando lo schema `YYYY/MM/DD`
>
> * **Funzioni chiave da implementare:**
>
>   * `initConfig()` → Viper + TOML
>   * `watchFS(root string)` → fsnotify + `handleUpsert` + `handleDelete`
>   * `anduril import` is already developed somewhere else
>
> * **Obiettivi qualitativi:**
>
>   * Architettura **single‑binary**
>   * Configurazione minimale
>   * Manutenibilità a lungo termine, zero “quick & dirty”
>   
> * Folder tree
    
    - anduril/cmd
    - anduril/main.go
    - anduril/lib
    - anduril/config.toml

