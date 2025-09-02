# Anduril Test Suite

Cartelle di test per validare le funzionalità di Anduril.

## Struttura Test:

### `test/quality/`
**Obiettivo**: Testare il confronto di qualità tra immagini
- **Stesso contenuto, qualità diverse**: test_photo_high.jpg (95%), test_photo_medium.jpg (80%), test_photo_low.jpg (50%), test_photo_worst.jpg (20%)
- **Risoluzioni diverse**: 1920x1080, 1280x720, 640x480, 320x240
- **Script**: `create_images.go` per generare le immagini

**Test Atteso**: 
- Importando più volte, dovrebbe mantenere solo la qualità più alta
- File con risoluzione maggiore dovrebbero sostituire quelli con risoluzione minore

### `test/messages/`
**Obiettivo**: Testare il riconoscimento pattern filename da app messaggistica
- `signal_20240315_143022.jpg` (pattern Signal)
- `IMG-20240315-WA0001.jpg` (pattern WhatsApp) 
- `telegram_2024-03-15.jpg` (pattern Telegram)
- `InShot_20240315_143022.jpg` (pattern Instagram)
- `VID-20240315-WA0001.mp4` (video WhatsApp)

**Test Atteso**:
- File dovrebbero finire in `/2024/03/15/` invece di `/noexif/`
- Date estratte dai nomi file invece di modification time

### `test/mixed/`
**Obiettivo**: Testare che file non-media vengano ignorati
- `caz.txt` (documento)
- `README.md` (markdown)
- `20250702_080752.jpg` (immagine)
- `signal-2025-07-15-151253_002.mp4` (video)

**Test Atteso**:
- Solo immagini e video processati
- Documenti completamente ignorati (rimangono in posto)

## Esecuzione Test:

```bash
# Test completo automatico
./test_anduril.sh

# Test individuali 
anduril import test/quality --dry-run --user testuser
anduril import test/messages --dry-run --user testuser  
anduril import test/mixed --dry-run --user testuser
```

## Scenari da Verificare:

1. **Quality Check**: File di qualità inferiore non sostituiscono quelli migliori
2. **Pattern Recognition**: Date estratte correttamente dai nomi file app
3. **Non-Media Skip**: File documenti completamente ignorati
4. **Duplicate Handling**: Stessi file con qualità diverse gestiti intelligentemente
5. **Directory Structure**: File finiscono nelle cartelle giuste basate su confidenza data