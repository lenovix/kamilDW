Saya ingin membangun sebuah software desktop Download Manager modern berkinerja tinggi bernama "kamilDW" (alternatif modern untuk Internet Download Manager / IDM). 

Aplikasi ini berfokus pada kecepatan transfer maksimal, efisiensi resource (RAM < 60 MB), arsitektur modular, antarmuka modern yang bersih, serta integrasi browser mulus.

Berikut adalah spesifikasi teknis dan daftar fitur lengkap yang harus diimplementasikan:

### 1. TECH STACK
- **Core Engine & Backend:** Go (Golang)
  - Concurrency & Goroutines untuk multi-thread HTTP worker.
  - Low-level socket & streaming I/O (`net/http`, `os.File`, `io`).
  - Pre-allocated zero-merge writing via `File.WriteAt` / sparse files.
- **Desktop Framework:** Wails (v2/v3)
  - Native Webview binding (Edge WebView2 di Windows / WebKit di Linux & macOS).
  - Ringan tanpa overhead runtime Chromium penuh.
- **Frontend / UI:** React, TypeScript, Tailwind CSS, Shadcn UI, Lucide Icons.
- **State & Metadata Storage:** Embedded SQLite (pure-Go via `modernc.org/sqlite`).
- **Media Pipeline:** Bundled FFmpeg CLI binary untuk penggabungan fragmen HLS (`.m3u8` / `.ts`) ke `.mp4`.
- **Browser Extension:** WebExtensions API (Manifest V3) untuk Chromium-based browser dan Firefox, berkomunikasi ke desktop app via Localhost HTTP/WebSocket server (`127.0.0.1:19890`).

---

### 2. CORE FEATURES & SPESIFIKASI

#### A. Core Download Engine (High-Performance)
1. **Dynamic Multi-part Segmentation:**
   - Pre-flight check via HTTP `HEAD` untuk memvalidasi `Accept-Ranges: bytes` dan `Content-Length`.
   - Memecah unduhan menjadi 4 hingga 32 koneksi paralel secara dinamis.
   - Dynamic re-chunking: koneksi yang selesai lebih awal otomatis mengambil alih sisa rentang byte dari segmen yang paling lambat.
2. **Zero-Merge Disk Writing:**
   - Mengalokasikan ukuran total file di disk sejak awal.
   - Setiap worker menulis langsung ke offset byte masing-masing (`WriteAt`), menghindari proses penggabungan (merge) file sementara yang lambat di akhir download.
3. **Resilience & State Persistence:**
   - Menyimpan checkpoint byte unduhan ke SQLite secara real-time.
   - Mendukung penuh pause, resume, dan auto-retry saat koneksi timeout atau putus.
4. **Traffic & Bandwidth Control:**
   - Speed limiter global dan per-task menggunakan algoritma Token Bucket.
   - Konfigurasi batas unduhan aktif simultan.

#### B. Browser Integration & Media Sniffing
1. **Download Interception:** Otomatis menangkap link unduhan saat pengguna mengklik tautan unduh di browser.
2. **Media Sniffer Floating Widget:**
   - Mendeteksi request audio/video di halaman web (format statis MP4/WebM dan streaming manifest HLS `.m3u8` / MPEG-DASH `.mpd`).
   - Memunculkan tombol popup floating "Download Video dengan kamilDW" lengkap dengan pilihan resolusi.
3. **Context Menu:** Klik kanan link apa saja di browser -> "Download with kamilDW".

#### C. User Interface & Task Management (Desktop Dashboard)
1. **Real-Time Visual Metrics:**
   - Grafik kecepatan download real-time (KB/s, MB/s) dan estimasi sisa waktu (ETA).
   - Bar visualisasi segmen/chunk (menampilkan progress individual worker koneksi mirip IDM klasik namun dengan gaya modern).
2. **Queue & Scheduling:**
   - Manajemen antrean unduhan (drag-and-drop prioritas, start/stop antrean otomatis pada jam tertentu).
3. **Smart Categorization:**
   - Otomatis memindahkan file selesai ke folder khusus berdasarkan ekstensi (Compressed, Documents, Music, Videos, Programs).
4. **Clipboard Listener:**
   - Deteksi otomatis link URL valid saat disalin ke clipboard pengguna dan memunculkan dialog tambah unduhan baru.

---

Berdasarkan spesifikasi di atas, buatkan:
1. Struktur folder modular untuk project kamilDW (Go backend + Wails + React frontend).
2. Skema database SQLite untuk mencatat antrean tugas (`tasks`) dan progress segmen (`chunks`).
3. Implementasi Go awal untuk engine download multi-part dengan `WriteAt`.