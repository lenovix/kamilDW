# kamilDW

Aplikasi Download Manager modern alternatif Internet Download Manager (IDM) berbasis **Go**, **SQLite**, **React**, dan **Wails**.

---

## 🚀 Fitur Utama

- **Turbo Multi-part Engine:** 4–32 goroutines paralel dengan zero-merge disk writing (`WriteAt`).
- **Dynamic Re-chunking & Auto-retry:** Segmen lambat otomatis di-split ulang, auto-retry saat timeout.
- **Bandwidth Control:** Global speed limiter berbasis algoritma Token Bucket.
- **Clipboard Listener:** Deteksi otomatis link URL yang disalin ke clipboard.
- **Browser Extension:** Tangkap unduhan otomatis & media sniffer floating widget.
- **Desktop Platform:** Native WebView2 via Wails v2 (Windows).
- **Web Platform:** Headless mode via `--web` flag, akses dari browser.
- **Real-time Logging:** Log startup, error, crash, dan lifecycle ke `~/.kamildw/kamildw.log`.
- **Modern UI:** React, TypeScript, Tailwind CSS dengan visualizer segmen real-time.

---

## 🛠️ Persyaratan Sistem

- **Go** (v1.24 atau lebih baru)
- **Node.js** & **npm** (untuk build UI frontend)
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **WebView2** (Windows, sudah built-in di Windows 11)
- **FFmpeg** (opsional, untuk merge HLS `.m3u8`)

---

## 📦 Cara Build & Jalankan

### 1. Build Frontend UI
```bash
cd frontend
npm install
npm run build
cd ..
```

### 2. Build Desktop App (Wails)
```bash
wails build -o kamilDW.exe
build\bin\kamilDW.exe
```

### 3. Build Web/Headless Mode
```bash
go build -o build/kamilDW.exe .
build\kamilDW.exe --web
```
Buka browser: `http://127.0.0.1:19890`

### 4. Development Mode

**Desktop (Wails dev):**
```bash
wails dev
```

**Web (headless):**
```bash
go run . --web
```

**Frontend dev server (hot reload):**
```bash
cd frontend
npm run dev
```
Buka `http://localhost:3000` (proxy `/api` ke `127.0.0.1:19890`).

---

## 🌐 Pasang Extension Browser (Chrome / Edge / Firefox)

1. Buka `chrome://extensions/` di browser.
2. Aktifkan **Developer mode** (Mode pengembang) di sudut kanan atas.
3. Klik **Load unpacked** (Muat yang dibuka).
4. Pilih folder `extension/` di project kamilDW.

---

## 📂 Struktur Project

```text
kamilDW/
├── build/             # Executable terkompilasi
│   ├── kamilDW.exe    # Desktop/Web binary
│   └── bin/           # Output wails build
├── extension/         # Manifest V3 extension (background & media sniffer)
├── frontend/          # React + Vite + Tailwind UI
│   ├── dist/          # Build output (embedded ke binary)
│   └── src/           # Source code
├── internal/
│   ├── app/           # Service orchestration layer
│   ├── clipboard/     # Windows clipboard listener
│   ├── engine/        # Core multi-part HTTP engine & zero-merge writer
│   ├── extserver/     # HTTP/SSE server (127.0.0.1:19890)
│   ├── limiter/       # Token Bucket speed limiter
│   ├── media/         # FFmpeg HLS runner
│   └── store/         # SQLite DAO (tasks & chunks)
├── go.mod
├── main.go            # Entrypoint (desktop + web mode)
├── wails.json         # Wails config
└── README.md
```

---

## 📝 Log File

Log ditulis ke `~/.kamildw/kamildw.log`:

- Startup: path DB, direktori download, status service & server
- Lifecycle: `OnStartup`, `OnDomReady`, `OnShutdown`
- Error: service init gagal, server gagal bind, wails gagal buka
- Crash: panic ditangkap + full stack trace
