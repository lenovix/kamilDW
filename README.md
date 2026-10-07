# kamilDW

Aplikasi Download Manager modern alternatif Internet Download Manager (IDM) berbasis **Go**, **SQLite**, dan **React**.

---

## 🚀 Fitur Utama

- **Turbo Multi-part Engine:** 4–32 goroutines paralel dengan zero-merge disk writing (`WriteAt`).
- **Dynamic Re-chunking & Auto-retry:** Segmen lambat otomatis di-split ulang, auto-retry saat timeout.
- **Bandwidth Control:** Global speed limiter berbasis algoritma Token Bucket.
- **Clipboard Listener:** Deteksi otomatis link URL yang disalin ke clipboard.
- **Browser Extension:** Tangkap unduhan otomatis & media sniffer floating widget.
- **Modern UI:** React, TypeScript, Tailwind CSS dengan visualizer segmen real-time.

---

## 🛠️ Persyaratan Sistem

- **Go** (v1.22 atau lebih baru)
- **Node.js** & **npm** (untuk build UI frontend)
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

### 2. Build & Jalankan Engine Backend
```bash
go mod tidy
go build -o build/kamildw.exe .
build\kamildw.exe
```

Aplikasi backend & web dashboard akan berjalan di `http://127.0.0.1:19890`.

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
├── extension/         # Manifest V3 extension (background & media sniffer)
├── frontend/          # React + Vite + Tailwind UI
├── internal/
│   ├── app/           # Service orchestration layer
│   ├── clipboard/     # Windows clipboard listener
│   ├── engine/        # Core multi-part HTTP engine & zero-merge writer
│   ├── extserver/     # HTTP/SSE server (127.0.0.1:19890)
│   ├── limiter/       # Token Bucket speed limiter
│   ├── media/         # FFmpeg HLS runner
│   └── store/         # SQLite DAO (tasks & chunks)
├── go.mod
├── main.go            # Entrypoint utama
└── README.md
```
