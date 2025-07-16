# Anduril

![anduril](anduril.jpg)

**Anduril** is a lightweight and robust Go CLI tool to ingest, sort, and de-duplicate large collections of media files (photos and videos), especially from mobile phones, SD cards, or disorganized backup drives. It organizes media into a structured destination library based on their creation dates, while intelligently handling duplicates, low-quality copies, and missing metadata.

**Use case**:
Import thousands of images from phones, old USB drives, or hard disks and organize them in a human-friendly structure without loss, duplication, or overwriting.

---

### 📁 Output Structure

```
$LIBRARY/
├── 2024/
│   ├── 07/
│   │   ├── 14/
│   │   │   └── IMG_001.jpg
├── noexif/
│   ├── IMG_567_2.jpg
```

---

### 🔑 Features

* ✅ Organizes media files into: `$LIBRARY/year/month/day/filename`
* ✅ Detects and avoids **exact duplicates** (using content hashing)
* ✅ Keeps **highest quality** version if multiple exist
* ✅ Falls back to file mtime or filename pattern when EXIF is missing
* ✅ Sends metadata-less media to a `noexif/` folder
* ✅ Renames conflicts as `filename_2.ext`, `filename_3.ext`, etc.
* ✅ Supports `.jpg`, `.jpeg`, `.png`, `.mov`, `.mp4`, `.heic`, etc.
* ✅ Logs every operation: copy, skip, rename, conflict

---

### ⚙️ Planned Flags and Options

```sh
anduril -src /mnt/phone -dst ~/Pictures/library \
  [--dry-run] [--move] [--verbose] [--prefer-mtime] \
  [--max-workers 4] [--skip-raw] [--timezone-adjust +2h]
```

### Future goals

- [ ] concurrency
