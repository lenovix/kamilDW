import { useState, useEffect } from 'react';
import {
  Download,
  Pause,
  Play,
  X,
  Plus,
  Folder,
  FileArchive,
  FileText,
  Music,
  Video,
  Cpu,
  Layers,
  Activity,
  Gauge,
  CheckCircle2,
  HardDrive
} from 'lucide-react';

interface Chunk {
  idx: number;
  startOffset: number;
  endOffset: number;
  downloaded: number;
  done: boolean;
}

interface Task {
  id: string;
  url: string;
  filename: string;
  filePath: string;
  totalBytes: number;
  downloaded: number;
  status: 'queued' | 'downloading' | 'paused' | 'completed' | 'failed' | 'canceled';
  connections: number;
  category: string;
  speed?: number;
  chunks?: Chunk[];
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

function formatSpeed(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec)}/s`;
}

export default function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<string>('All');
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [urlInput, setUrlInput] = useState('');
  const [threads, setThreads] = useState(8);
  const [speedLimit, setSpeedLimit] = useState(0); // 0 = unlimited

  // Fetch / Poll updates from engine
  useEffect(() => {
    // SSE listener for real-time downloads from browser extension or engine
    const eventSource = new EventSource('http://127.0.0.1:19890/api/events');
    eventSource.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload.url) {
          // Auto add task to UI
          setTasks((prev) => [
            {
              id: Math.random().toString(36).substring(2, 9),
              url: payload.url,
              filename: payload.filename || 'download.bin',
              filePath: 'C:/Downloads/' + (payload.filename || 'download.bin'),
              totalBytes: 104857600, // sample 100MB
              downloaded: 0,
              status: 'downloading',
              connections: 8,
              category: 'Other',
              speed: 2500000,
              chunks: Array.from({ length: 8 }, (_, i) => ({
                idx: i,
                startOffset: i * 13107200,
                endOffset: (i + 1) * 13107200 - 1,
                downloaded: 0,
                done: false
              }))
            },
            ...prev
          ]);
        }
      } catch (err) {
        console.error(err);
      }
    };

    return () => eventSource.close();
  }, []);

  // Demo progress simulation loop for visual metrics
  useEffect(() => {
    const interval = setInterval(() => {
      setTasks((prev) =>
        prev.map((t) => {
          if (t.status !== 'downloading') return t;
          const chunkAdd = Math.floor(Math.random() * 400000) + 100000;
          let newDownloaded = t.downloaded + chunkAdd * (t.chunks?.length || 1);
          if (t.totalBytes > 0 && newDownloaded >= t.totalBytes) {
            newDownloaded = t.totalBytes;
            return { ...t, downloaded: newDownloaded, status: 'completed', speed: 0 };
          }
          const updatedChunks = t.chunks?.map((c) => {
            const added = Math.min(chunkAdd, c.endOffset - c.startOffset - c.downloaded);
            return {
              ...c,
              downloaded: c.downloaded + added,
              done: c.downloaded + added >= c.endOffset - c.startOffset
            };
          });
          return {
            ...t,
            downloaded: newDownloaded,
            speed: chunkAdd * (t.chunks?.length || 1),
            chunks: updatedChunks
          };
        })
      );
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  const totalSpeed = tasks
    .filter((t) => t.status === 'downloading')
    .reduce((acc, t) => acc + (t.speed || 0), 0);

  const filteredTasks = tasks.filter((t) => {
    if (selectedCategory === 'All') return true;
    return t.category.toLowerCase() === selectedCategory.toLowerCase();
  });

  const handleAddDownload = () => {
    if (!urlInput) return;
    const filename = urlInput.split('/').pop()?.split('?')[0] || 'file.bin';
    const totalBytes = 52428800; // 50MB dummy
    const chunkSize = Math.floor(totalBytes / threads);

    const newTask: Task = {
      id: Math.random().toString(36).substring(2, 9),
      url: urlInput,
      filename: filename,
      filePath: `C:/Downloads/${filename}`,
      totalBytes: totalBytes,
      downloaded: 0,
      status: 'downloading',
      connections: threads,
      category: 'Compressed',
      speed: 1500000,
      chunks: Array.from({ length: threads }, (_, i) => ({
        idx: i,
        startOffset: i * chunkSize,
        endOffset: i === threads - 1 ? totalBytes - 1 : (i + 1) * chunkSize - 1,
        downloaded: 0,
        done: false
      }))
    };

    setTasks([newTask, ...tasks]);
    setUrlInput('');
    setIsAddOpen(false);
  };

  const togglePause = (id: string) => {
    setTasks((prev) =>
      prev.map((t) => {
        if (t.id === id) {
          return {
            ...t,
            status: t.status === 'downloading' ? 'paused' : 'downloading',
            speed: t.status === 'downloading' ? 0 : 1500000
          };
        }
        return t;
      })
    );
  };

  const removeTask = (id: string) => {
    setTasks((prev) => prev.filter((t) => t.id !== id));
  };

  return (
    <div className="flex h-screen w-full bg-slate-950 text-slate-100 select-none overflow-hidden font-sans">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900/70 border-r border-slate-800 flex flex-col justify-between p-4 backdrop-blur-md">
        <div>
          {/* Logo & Header */}
          <div className="flex items-center gap-3 px-2 py-3 mb-6">
            <div className="h-10 w-10 rounded-xl bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
              <Download className="w-6 h-6 text-white" />
            </div>
            <div>
              <h1 className="font-bold text-lg leading-tight tracking-wide text-white">kamilDW</h1>
              <p className="text-xs text-sky-400 font-medium">Turbo Downloader</p>
            </div>
          </div>

          {/* Action Button */}
          <button
            onClick={() => setIsAddOpen(true)}
            className="w-full mb-6 bg-sky-500 hover:bg-sky-400 text-white font-medium py-2.5 px-4 rounded-xl flex items-center justify-center gap-2 shadow-lg shadow-sky-500/20 transition active:scale-[0.98]"
          >
            <Plus className="w-5 h-5" />
            <span>Tambah Unduhan</span>
          </button>

          {/* Categories Nav */}
          <nav className="space-y-1">
            {[
              { label: 'All', icon: Layers, count: tasks.length },
              { label: 'Compressed', icon: FileArchive },
              { label: 'Documents', icon: FileText },
              { label: 'Music', icon: Music },
              { label: 'Videos', icon: Video },
              { label: 'Programs', icon: Cpu }
            ].map(({ label, icon: Icon, count }) => (
              <button
                key={label}
                onClick={() => setSelectedCategory(label)}
                className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm transition font-medium ${
                  selectedCategory === label
                    ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20'
                    : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                }`}
              >
                <div className="flex items-center gap-3">
                  <Icon className="w-4 h-4" />
                  <span>{label}</span>
                </div>
                {count !== undefined && count > 0 && (
                  <span className="text-xs bg-slate-800 px-2 py-0.5 rounded-full text-slate-300">
                    {count}
                  </span>
                )}
              </button>
            ))}
          </nav>
        </div>

        {/* Global Speedometer Widget */}
        <div className="bg-slate-900/90 border border-slate-800/80 rounded-xl p-3.5 space-y-3 shadow-inner">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5 font-medium">
              <Activity className="w-3.5 h-3.5 text-emerald-400 animate-pulse" />
              Total Speed
            </span>
            <span className="text-emerald-400 font-mono font-semibold">{formatSpeed(totalSpeed)}</span>
          </div>

          <div className="flex items-center justify-between text-xs text-slate-400 pt-1 border-t border-slate-800">
            <span className="flex items-center gap-1.5">
              <Gauge className="w-3.5 h-3.5 text-sky-400" />
              Limit: {speedLimit === 0 ? 'Unlimited' : `${speedLimit} MB/s`}
            </span>
            <input
              type="range"
              min="0"
              max="50"
              value={speedLimit}
              onChange={(e) => setSpeedLimit(Number(e.target.value))}
              className="w-20 accent-sky-500 h-1.5 bg-slate-800 rounded-lg cursor-pointer"
            />
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col bg-slate-950 overflow-hidden">
        {/* Top bar */}
        <header className="h-16 border-b border-slate-800/80 flex items-center justify-between px-6 bg-slate-900/30 backdrop-blur">
          <div className="flex items-center gap-3">
            <h2 className="text-base font-semibold text-slate-100">{selectedCategory} Downloads</h2>
            <span className="text-xs text-slate-500 bg-slate-800 px-2.5 py-0.5 rounded-full">
              {filteredTasks.length} items
            </span>
          </div>

          <div className="flex items-center gap-4 text-xs text-slate-400">
            <div className="flex items-center gap-1.5 bg-slate-900 border border-slate-800 px-3 py-1.5 rounded-lg">
              <HardDrive className="w-4 h-4 text-sky-400" />
              <span>Free Disk: 142.8 GB</span>
            </div>
          </div>
        </header>

        {/* Task List */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          {filteredTasks.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-slate-500 gap-3">
              <Folder className="w-12 h-12 stroke-[1.5] text-slate-600" />
              <p className="text-sm">Tidak ada unduhan di kategori ini</p>
            </div>
          ) : (
            filteredTasks.map((t) => {
              const progressPct = t.totalBytes > 0 ? (t.downloaded / t.totalBytes) * 100 : 0;
              return (
                <div
                  key={t.id}
                  className="bg-slate-900/60 border border-slate-800/80 hover:border-slate-700/80 rounded-xl p-4 transition shadow-sm space-y-3"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3 overflow-hidden">
                      <div className="p-2.5 bg-slate-800 rounded-lg text-sky-400">
                        <Download className="w-5 h-5" />
                      </div>
                      <div className="truncate">
                        <h4 className="font-semibold text-sm text-slate-200 truncate">{t.filename}</h4>
                        <p className="text-xs text-slate-400 truncate mt-0.5">{t.url}</p>
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center gap-2">
                      {t.status !== 'completed' && (
                        <button
                          onClick={() => togglePause(t.id)}
                          className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition"
                        >
                          {t.status === 'downloading' ? (
                            <Pause className="w-4 h-4" />
                          ) : (
                            <Play className="w-4 h-4" />
                          )}
                        </button>
                      )}
                      <button
                        onClick={() => removeTask(t.id)}
                        className="p-2 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 transition"
                      >
                        <X className="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  {/* Main Progress Bar */}
                  <div>
                    <div className="flex justify-between text-xs font-mono text-slate-400 mb-1.5">
                      <span className="flex items-center gap-1.5">
                        {t.status === 'completed' ? (
                          <span className="text-emerald-400 flex items-center gap-1 font-sans">
                            <CheckCircle2 className="w-3.5 h-3.5" /> Selesai
                          </span>
                        ) : (
                          <span>
                            {formatBytes(t.downloaded)} / {formatBytes(t.totalBytes)} ({progressPct.toFixed(1)}%)
                          </span>
                        )}
                      </span>
                      {t.status === 'downloading' && (
                        <span className="text-emerald-400 font-semibold">{formatSpeed(t.speed || 0)}</span>
                      )}
                    </div>
                    <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
                      <div
                        className="bg-gradient-to-r from-sky-500 to-indigo-500 h-full rounded-full transition-all duration-300"
                        style={{ width: `${progressPct}%` }}
                      />
                    </div>
                  </div>

                  {/* Dynamic Multi-Part Segmentation Visualizer (IDM Style) */}
                  {t.chunks && t.chunks.length > 0 && t.status !== 'completed' && (
                    <div className="pt-2 border-t border-slate-800/60">
                      <div className="flex items-center justify-between text-[11px] text-slate-500 mb-1.5">
                        <span className="font-mono">
                          Segmen Multi-Part ({t.chunks.length} Threads Paralel)
                        </span>
                      </div>
                      <div className="grid grid-cols-8 gap-1.5">
                        {t.chunks.map((c) => {
                          const segTotal = c.endOffset - c.startOffset;
                          const segPct = segTotal > 0 ? (c.downloaded / segTotal) * 100 : 0;
                          return (
                            <div
                              key={c.idx}
                              className="bg-slate-800/80 rounded h-3 overflow-hidden border border-slate-700/40 relative group"
                              title={`Thread #${c.idx + 1}: ${segPct.toFixed(0)}%`}
                            >
                              <div
                                className="bg-sky-400 h-full transition-all duration-200"
                                style={{ width: `${segPct}%` }}
                              />
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      </main>

      {/* Add Download Modal */}
      {isAddOpen && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg p-6 shadow-2xl space-y-5">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-lg text-slate-100 flex items-center gap-2">
                <Plus className="w-5 h-5 text-sky-400" />
                Tambah Unduhan Baru
              </h3>
              <button
                onClick={() => setIsAddOpen(false)}
                className="text-slate-400 hover:text-white p-1 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  URL Unduhan (HTTP / HTTPS / HLS)
                </label>
                <input
                  type="text"
                  placeholder="https://example.com/file.zip"
                  value={urlInput}
                  onChange={(e) => setUrlInput(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-600 focus:outline-none focus:border-sky-500 transition"
                  autoFocus
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Koneksi Paralel (Goroutine Segments): {threads} Threads
                </label>
                <input
                  type="range"
                  min="4"
                  max="32"
                  step="4"
                  value={threads}
                  onChange={(e) => setThreads(Number(e.target.value))}
                  className="w-full accent-sky-500 h-2 bg-slate-800 rounded-lg cursor-pointer"
                />
                <div className="flex justify-between text-[11px] text-slate-500 mt-1">
                  <span>4 Threads</span>
                  <span>16 Threads</span>
                  <span>32 Threads (Max)</span>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                onClick={() => setIsAddOpen(false)}
                className="px-4 py-2 rounded-xl text-sm font-medium text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition"
              >
                Batal
              </button>
              <button
                onClick={handleAddDownload}
                className="px-5 py-2 rounded-xl text-sm font-medium bg-sky-500 hover:bg-sky-400 text-white shadow-lg shadow-sky-500/20 transition"
              >
                Mulai Download
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
