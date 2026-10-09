import { useState, useEffect } from 'react';
import {
  Download,
  Clock,
  CheckCircle2,
  Folder,
  FileText,
  FileArchive,
  Music,
  Video,
  LayoutGrid,
  Plus,
  Trash2,
  ArrowUpDown,
  Settings,
  Search,
  Menu,
  FolderOpen,
  MoreVertical,
  Pause,
  Play,
  File,
  X,
  Database,
  RefreshCw
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
  updatedAt?: string;
}

interface DBTask {
  id: string;
  url: string;
  filename: string;
  filePath: string;
  totalBytes: number;
  downloaded: number;
  status: string;
  connections: number;
  category: string;
  etag: string;
  acceptRanges: boolean;
  error: string;
  createdAt: string;
  updatedAt: string;
}

interface DBChunk {
  id: number;
  taskId: string;
  idx: number;
  startOffset: number;
  endOffset: number;
  downloaded: number;
  status: string;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec <= 0) return '0 B/s';
  return `${formatBytes(bytesPerSec)}/s`;
}

function formatETA(seconds: number): string {
  if (!seconds || seconds <= 0 || !isFinite(seconds)) return '0s left';
  if (seconds < 60) return `${Math.ceil(seconds)}s left`;
  const mins = Math.floor(seconds / 60);
  const secs = Math.ceil(seconds % 60);
  return `${mins}m ${secs}s left`;
}

function getCategoryIcon(category: string, filename: string) {
  const cat = category.toLowerCase();
  const ext = filename.split('.').pop()?.toLowerCase() || '';

  if (cat === 'compressed' || ['zip', 'tar', 'gz', 'xz', 'rar', '7z', 'bz2'].includes(ext)) {
    return <FileArchive className="w-5 h-5 text-white" />;
  }
  if (cat === 'documents' || ['pdf', 'doc', 'docx', 'txt', 'epub'].includes(ext)) {
    return <FileText className="w-5 h-5 text-white" />;
  }
  if (cat === 'music' || ['mp3', 'flac', 'wav', 'aac', 'm4a', 'ogg'].includes(ext)) {
    return <Music className="w-5 h-5 text-white" />;
  }
  if (cat === 'video' || ['mp4', 'mkv', 'webm', 'avi', 'mov', 'flv'].includes(ext)) {
    return <Video className="w-5 h-5 text-white" />;
  }
  if (cat === 'programs' || ['exe', 'msi', 'dmg', 'app', 'deb', 'rpm'].includes(ext)) {
    return <LayoutGrid className="w-5 h-5 text-white" />;
  }
  return <File className="w-5 h-5 text-white" />;
}

export default function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [statusFilter, setStatusFilter] = useState<'all' | 'incomplete' | 'completed'>('all');
  const [selectedCategory, setSelectedCategory] = useState<string>('All');
  const [searchQuery, setSearchQuery] = useState<string>('');

  // Modals
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [activeProgressTask, setActiveProgressTask] = useState<Task | null>(null);

  // Form Inputs for New Download Modal
  const [urlInput, setUrlInput] = useState('');
  const [filenameInput, setFilenameInput] = useState('');
  const [saveInInput, setSaveInInput] = useState('/Users/subhro/Downloads');
  const [threads, setThreads] = useState(8);
  const [probeSize, setProbeSize] = useState<number | null>(null);
  const [probeLoading, setProbeLoading] = useState(false);

  // View Mode
  const [viewMode, setViewMode] = useState<'downloads' | 'database'>('downloads');
  const [dbTab, setDbTab] = useState<'tasks' | 'chunks'>('tasks');
  const [dbTasks, setDbTasks] = useState<DBTask[]>([]);
  const [dbChunks, setDbChunks] = useState<DBChunk[]>([]);
  const [dbSearch, setDbSearch] = useState('');
  const [dbLoading, setDbLoading] = useState(false);

  const fetchDatabase = async () => {
    setDbLoading(true);
    try {
      const res = await fetch('http://127.0.0.1:19890/api/db');
      if (res.ok) {
        const data = await res.json();
        setDbTasks(Array.isArray(data.tasks) ? data.tasks : []);
        setDbChunks(Array.isArray(data.chunks) ? data.chunks : []);
      }
    } catch {
      // offline
    } finally {
      setDbLoading(false);
    }
  };

  const fetchTasks = async () => {
    try {
      const res = await fetch('http://127.0.0.1:19890/api/tasks');
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data)) {
          setTasks((prev) => {
            const map = new Map(prev.map((t) => [t.id, t]));
            return data.map((t: Task) => ({
              ...t,
              speed: map.get(t.id)?.speed || 0,
              chunks: map.get(t.id)?.chunks || []
            }));
          });
        }
      }
    } catch {
      // offline / backend not started yet
    }
  };

  useEffect(() => {
    if (viewMode === 'database') {
      fetchDatabase();
    }
  }, [viewMode]);

  useEffect(() => {
    fetchTasks();

    const eventSource = new EventSource('http://127.0.0.1:19890/api/events');
    eventSource.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload.type === 'progress') {
          setTasks((prev) =>
            prev.map((t) => {
              if (t.id === payload.id) {
                const updated: Task = {
                  ...t,
                  downloaded: payload.downloaded,
                  speed: payload.speed,
                  chunks: payload.chunks,
                  status: payload.status
                };
                // Keep active progress modal in sync
                if (activeProgressTask && activeProgressTask.id === payload.id) {
                  setActiveProgressTask(updated);
                }
                return updated;
              }
              return t;
            })
          );
        } else if (payload.type === 'new_task') {
          fetchTasks();
        } else if (payload.type === 'task_deleted') {
          setTasks((prev) => prev.filter((t) => t.id !== payload.id));
          if (activeProgressTask && activeProgressTask.id === payload.id) {
            setActiveProgressTask(null);
          }
        }
      } catch (err) {
        console.error(err);
      }
    };

    return () => eventSource.close();
  }, [activeProgressTask]);

  // Derive filename & probe file size automatically when URL changes in New Download modal
  const handleUrlChange = async (val: string) => {
    setUrlInput(val);
    setProbeSize(null);
    if (val) {
      try {
        const parsed = new URL(val);
        const name = parsed.pathname.split('/').pop() || 'download.bin';
        if (name && name !== '/') {
          setFilenameInput(decodeURIComponent(name));
        }
      } catch {
        const parts = val.split('/');
        if (parts.length > 1 && parts[parts.length - 1]) {
          setFilenameInput(parts[parts.length - 1].split('?')[0]);
        }
      }

      if (val.startsWith('http://') || val.startsWith('https://')) {
        setProbeLoading(true);
        try {
          const res = await fetch('http://127.0.0.1:19890/api/probe', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: val })
          });
          if (res.ok) {
            const data = await res.json();
            if (data.total > 0) {
              setProbeSize(data.total);
            }
            if (data.filename && data.filename !== 'download.bin' && data.filename !== 'video.mp4') {
              setFilenameInput(data.filename);
            }
          }
        } catch {
          // Ignore probe errors
        } finally {
          setProbeLoading(false);
        }
      }
    }
  };

  const handleAddDownload = async () => {
    if (!urlInput) return;
    try {
      await fetch('http://127.0.0.1:19890/api/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: urlInput,
          filename: filenameInput,
          savePath: saveInInput,
          connections: threads
        })
      });
      setUrlInput('');
      setFilenameInput('');
      setIsAddOpen(false);
      fetchTasks();
    } catch (err) {
      console.error(err);
    }
  };

  const togglePause = async (t: Task, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    const action = t.status === 'downloading' ? 'pause' : 'resume';
    try {
      await fetch(`http://127.0.0.1:19890/api/tasks/${t.id}/${action}`, {
        method: 'POST'
      });
      fetchTasks();
    } catch (err) {
      console.error(err);
    }
  };

  const removeTask = async (id: string, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    try {
      await fetch(`http://127.0.0.1:19890/api/tasks/${id}`, {
        method: 'DELETE'
      });
      setTasks((prev) => prev.filter((t) => t.id !== id));
      if (activeProgressTask && activeProgressTask.id === id) {
        setActiveProgressTask(null);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const clearCompleted = () => {
    tasks.filter((t) => t.status === 'completed').forEach((t) => removeTask(t.id));
  };

  // Filtering tasks
  const filteredTasks = tasks.filter((t) => {
    if (statusFilter === 'incomplete' && t.status === 'completed') return false;
    if (statusFilter === 'completed' && t.status !== 'completed') return false;

    if (selectedCategory !== 'All') {
      if (t.category.toLowerCase() !== selectedCategory.toLowerCase()) {
        return false;
      }
    }

    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      return t.filename.toLowerCase().includes(q) || t.url.toLowerCase().includes(q);
    }

    return true;
  });

  return (
    <div className="flex h-screen w-full bg-[#1b1b1e] text-[#d1d1d6] select-none overflow-hidden font-sans">
      {/* Sidebar (Matching Image 1) */}
      <aside className="w-56 bg-[#212124] border-r border-[#2c2c2e] flex flex-col justify-between p-3">
        <div className="space-y-6">
          {/* Top Window Dots Padding & Brand */}
          <div className="flex items-center gap-2 px-2 pt-1 pb-2">
            <div className="w-3 h-3 rounded-full bg-[#ff5f56] border border-[#e0443e]" />
            <div className="w-3 h-3 rounded-full bg-[#ffbd2e] border border-[#dea123]" />
            <div className="w-3 h-3 rounded-full bg-[#27c93f] border border-[#1aab29]" />
          </div>

          {/* Status Filters */}
          <div className="space-y-0.5">
            <button
              onClick={() => { setStatusFilter('all'); setViewMode('downloads'); }}
              className={`w-full flex items-center gap-3 px-3 py-1.5 rounded-md text-sm font-medium transition ${
                statusFilter === 'all' && viewMode === 'downloads'
                  ? 'bg-[#2c2c2e] text-white'
                  : 'text-[#98989d] hover:bg-[#28282b] hover:text-white'
              }`}
            >
              <div className="w-5 h-5 rounded-full bg-[#3a3a3c] flex items-center justify-center">
                <Download className="w-3 h-3 text-white" />
              </div>
              <span>All Downloads</span>
            </button>

            <button
              onClick={() => { setStatusFilter('incomplete'); setViewMode('downloads'); }}
              className={`w-full flex items-center gap-3 px-3 py-1.5 rounded-md text-sm font-medium transition ${
                statusFilter === 'incomplete' && viewMode === 'downloads'
                  ? 'bg-[#2c2c2e] text-white'
                  : 'text-[#98989d] hover:bg-[#28282b] hover:text-white'
              }`}
            >
              <div className="w-5 h-5 rounded-full bg-[#3a3a3c] flex items-center justify-center">
                <Clock className="w-3 h-3 text-white" />
              </div>
              <span>Incomplete</span>
            </button>

            <button
              onClick={() => { setStatusFilter('completed'); setViewMode('downloads'); }}
              className={`w-full flex items-center gap-3 px-3 py-1.5 rounded-md text-sm font-medium transition ${
                statusFilter === 'completed' && viewMode === 'downloads'
                  ? 'bg-[#2c2c2e] text-white'
                  : 'text-[#98989d] hover:bg-[#28282b] hover:text-white'
              }`}
            >
              <div className="w-5 h-5 rounded-full bg-[#3a3a3c] flex items-center justify-center">
                <CheckCircle2 className="w-3 h-3 text-white" />
              </div>
              <span>Completed</span>
            </button>
          </div>

          {/* Divider */}
          <div className="h-px bg-[#2c2c2e] my-3 mx-2" />

          {/* Category List */}
          <div className="space-y-0.5">
            {[
              { label: 'All Downloads', catKey: 'All', icon: Folder },
              { label: 'Documents', catKey: 'Documents', icon: FileText },
              { label: 'Compressed', catKey: 'Compressed', icon: FileArchive },
              { label: 'Music', catKey: 'Music', icon: Music },
              { label: 'Video', catKey: 'Videos', icon: Video },
              { label: 'Programs', catKey: 'Programs', icon: LayoutGrid }
            ].map(({ label, catKey, icon: Icon }) => {
              const isActive = selectedCategory === catKey && viewMode === 'downloads';
              return (
                <button
                  key={label}
                  onClick={() => { setSelectedCategory(catKey); setViewMode('downloads'); }}
                  className={`w-full flex items-center gap-3 px-3 py-1.5 rounded-md text-sm font-medium relative transition ${
                    isActive
                      ? 'bg-[#2c2c2e] text-white'
                      : 'text-[#98989d] hover:bg-[#28282b] hover:text-white'
                  }`}
                >
                  {isActive && (
                    <div className="absolute left-0 top-1 bottom-1 w-1 bg-[#007aff] rounded-r" />
                  )}
                  <Icon className="w-4 h-4 text-[#98989d]" />
                  <span>{label}</span>
                </button>
              );
            })}
          </div>

          {/* DB Inspector Access */}
          <div className="pt-2">
            <button
              onClick={() => setViewMode('database')}
              className={`w-full flex items-center gap-3 px-3 py-1.5 rounded-md text-sm font-medium transition ${
                viewMode === 'database'
                  ? 'bg-[#2c2c2e] text-white'
                  : 'text-[#98989d] hover:bg-[#28282b] hover:text-white'
              }`}
            >
              <Database className="w-4 h-4 text-[#98989d]" />
              <span>DB Inspector</span>
            </button>
          </div>
        </div>
      </aside>

      {/* Main Container */}
      <main className="flex-1 flex flex-col bg-[#1e1e20] overflow-hidden">
        {/* Top Window Bar (Matching Image 1) */}
        <header className="h-14 border-b border-[#2c2c2e] flex items-center justify-between px-4 bg-[#212124] relative">
          {/* Centered Title */}
          <h2 className="absolute left-1/2 -translate-x-1/2 text-sm font-semibold text-[#e1e1e6] pointer-events-none">
            kamilDW - Download Manager
          </h2>

          {/* Toolbar Action Buttons */}
          <div className="flex items-center gap-4 text-sm font-medium text-[#d1d1d6]">
            <button
              onClick={() => { setUrlInput(''); setFilenameInput(''); setIsAddOpen(true); }}
              className="flex items-center gap-1.5 hover:text-white transition"
            >
              <Plus className="w-4 h-4 text-[#007aff]" />
              <span>New</span>
            </button>

            <button
              onClick={clearCompleted}
              className="flex items-center gap-1.5 hover:text-white transition"
            >
              <Trash2 className="w-4 h-4 text-[#98989d]" />
              <span>Clear</span>
            </button>

            <button className="flex items-center gap-1.5 hover:text-white transition">
              <ArrowUpDown className="w-4 h-4 text-[#98989d]" />
              <span>Sort</span>
            </button>

            <button className="flex items-center gap-1.5 hover:text-white transition">
              <Settings className="w-4 h-4 text-[#98989d]" />
              <span>Settings</span>
            </button>
          </div>

          {/* Search Bar & Menu */}
          <div className="flex items-center gap-3">
            <div className="relative">
              <input
                type="text"
                placeholder="Search"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-48 bg-[#18181a] border border-[#3a3a3c] rounded-md pl-3 pr-8 py-1 text-xs text-white placeholder-[#6e6e73] focus:outline-none focus:border-[#007aff]"
              />
              <Search className="w-3.5 h-3.5 text-[#6e6e73] absolute right-2.5 top-1/2 -translate-y-1/2" />
            </div>

            <button className="p-1 text-[#98989d] hover:text-white">
              <Menu className="w-5 h-5" />
            </button>
          </div>
        </header>

        {/* Window Content */}
        {viewMode === 'downloads' ? (
          <div className="flex-1 overflow-y-auto p-4 space-y-2">
            {filteredTasks.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-[#6e6e73] gap-2 pt-20">
                <Folder className="w-12 h-12 stroke-[1]" />
                <p className="text-sm">No downloads found</p>
              </div>
            ) : (
              filteredTasks.map((t) => {
                const progressPct = t.totalBytes > 0 ? (t.downloaded / t.totalBytes) * 100 : 0;
                const isDone = t.status === 'completed';
                const isDownloading = t.status === 'downloading';

                return (
                  <div
                    key={t.id}
                    onClick={() => {
                      if (!isDone) setActiveProgressTask(t);
                    }}
                    className="bg-[#262629] hover:bg-[#2c2c30] border border-[#323236] rounded-lg p-3 flex items-center justify-between transition cursor-pointer group"
                  >
                    {/* File Icon & Info */}
                    <div className="flex items-center gap-3 min-w-0 pr-4">
                      <div className="w-9 h-9 rounded-full bg-[#007aff] flex items-center justify-center shrink-0 shadow-md">
                        {getCategoryIcon(t.category, t.filename)}
                      </div>
                      <div className="min-w-0">
                        <h4 className="text-sm font-medium text-white truncate leading-tight" title={t.filename}>
                          {t.filename}
                        </h4>
                        <p className="text-xs text-[#8e8e93] mt-0.5 font-sans">
                          Sept 22 - {formatBytes(t.totalBytes > 0 ? t.totalBytes : t.downloaded)}
                        </p>
                      </div>
                    </div>

                    {/* Progress / Actions (Matching Image 1) */}
                    <div className="flex items-center gap-4 shrink-0">
                      {!isDone && (
                        <div className="flex items-center gap-3">
                          <span className="text-xs text-[#8e8e93] font-medium">
                            {isDownloading ? `Downloading ${progressPct.toFixed(0)}%` : `Paused ${progressPct.toFixed(0)}%`}
                          </span>

                          <button
                            onClick={(e) => togglePause(t, e)}
                            className="p-1 text-[#d1d1d6] hover:text-white transition"
                          >
                            {isDownloading ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4 fill-current" />}
                          </button>
                        </div>
                      )}

                      {isDone && (
                        <button className="p-1 text-[#8e8e93] hover:text-white transition" title="Open Folder">
                          <FolderOpen className="w-4.5 h-4.5" />
                        </button>
                      )}

                      <button
                        onClick={(e) => removeTask(t.id, e)}
                        className="p-1 text-[#8e8e93] hover:text-rose-400 transition"
                        title="Delete"
                      >
                        <Trash2 className="w-4.5 h-4.5" />
                      </button>

                      <button className="p-1 text-[#8e8e93] hover:text-white transition">
                        <MoreVertical className="w-4.5 h-4.5" />
                      </button>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        ) : (
          /* DB Inspector View */
          <div className="flex-1 flex flex-col h-full bg-[#1e1e20]">
            <div className="px-4 py-3 border-b border-[#2c2c2e] flex items-center justify-between bg-[#212124]">
              <div className="flex gap-2">
                <button
                  onClick={() => setDbTab('tasks')}
                  className={`px-3 py-1 rounded text-xs font-medium transition ${
                    dbTab === 'tasks' ? 'bg-[#007aff] text-white' : 'bg-[#2c2c2e] text-[#8e8e93]'
                  }`}
                >
                  Tasks ({dbTasks.length})
                </button>
                <button
                  onClick={() => setDbTab('chunks')}
                  className={`px-3 py-1 rounded text-xs font-medium transition ${
                    dbTab === 'chunks' ? 'bg-[#007aff] text-white' : 'bg-[#2c2c2e] text-[#8e8e93]'
                  }`}
                >
                  Chunks ({dbChunks.length})
                </button>
              </div>

              <div className="flex items-center gap-3">
                <input
                  type="text"
                  placeholder="Search ID..."
                  value={dbSearch}
                  onChange={(e) => setDbSearch(e.target.value)}
                  className="bg-[#18181a] border border-[#3a3a3c] rounded px-2.5 py-1 text-xs text-white"
                />
                <button
                  onClick={fetchDatabase}
                  disabled={dbLoading}
                  className="p-1 text-[#8e8e93] hover:text-white"
                >
                  <RefreshCw className={`w-4 h-4 ${dbLoading ? 'animate-spin' : ''}`} />
                </button>
              </div>
            </div>

            <div className="flex-1 overflow-auto p-4">
              <table className="w-full text-left text-xs border-collapse">
                <thead className="bg-[#252528] text-[#8e8e93]">
                  {dbTab === 'tasks' ? (
                    <tr>
                      <th className="p-2">ID</th>
                      <th className="p-2">Filename</th>
                      <th className="p-2">Status</th>
                      <th className="p-2">Size</th>
                      <th className="p-2">Updated</th>
                    </tr>
                  ) : (
                    <tr>
                      <th className="p-2">ID</th>
                      <th className="p-2">Task ID</th>
                      <th className="p-2">Idx</th>
                      <th className="p-2">Status</th>
                      <th className="p-2">Range</th>
                      <th className="p-2">Downloaded</th>
                    </tr>
                  )}
                </thead>
                <tbody className="divide-y divide-[#2c2c2e]">
                  {dbTab === 'tasks' ? (
                    dbTasks
                      .filter(t => t.id.toLowerCase().includes(dbSearch.toLowerCase()) || t.filename.toLowerCase().includes(dbSearch.toLowerCase()))
                      .map(t => (
                        <tr key={t.id} className="hover:bg-[#252528]">
                          <td className="p-2 font-mono text-[#007aff]">{t.id.substring(0, 8)}...</td>
                          <td className="p-2 text-white">{t.filename}</td>
                          <td className="p-2 uppercase">{t.status}</td>
                          <td className="p-2">{formatBytes(t.downloaded)} / {formatBytes(t.totalBytes)}</td>
                          <td className="p-2 text-[#8e8e93]">{new Date(t.updatedAt).toLocaleTimeString()}</td>
                        </tr>
                      ))
                  ) : (
                    dbChunks
                      .filter(c => c.taskId.toLowerCase().includes(dbSearch.toLowerCase()))
                      .map(c => (
                        <tr key={`${c.taskId}-${c.idx}`} className="hover:bg-[#252528]">
                          <td className="p-2">{c.id || '-'}</td>
                          <td className="p-2 font-mono text-[#007aff]">{c.taskId.substring(0, 8)}...</td>
                          <td className="p-2">#{c.idx}</td>
                          <td className="p-2 uppercase">{c.status}</td>
                          <td className="p-2 font-mono">{c.startOffset} - {c.endOffset}</td>
                          <td className="p-2 font-mono">{formatBytes(c.downloaded)}</td>
                        </tr>
                      ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </main>

      {/* New Download Dialog Window (Matching Image 2) */}
      {isAddOpen && (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-[#2d2d30] border border-[#3e3e42] rounded-xl w-full max-w-lg shadow-2xl overflow-hidden font-sans">
            {/* macOS Modal Header */}
            <div className="h-9 bg-[#252528] border-b border-[#38383c] flex items-center justify-between px-3 relative">
              <div className="flex items-center gap-2">
                <button onClick={() => setIsAddOpen(false)} className="w-3 h-3 rounded-full bg-[#ff5f56] border border-[#e0443e]" />
                <div className="w-3 h-3 rounded-full bg-[#ffbd2e] border border-[#dea123]" />
                <div className="w-3 h-3 rounded-full bg-[#27c93f] border border-[#1aab29]" />
              </div>
              <h3 className="text-xs font-semibold text-[#e1e1e6] absolute left-1/2 -translate-x-1/2">
                New download
              </h3>
              <div />
            </div>

            {/* Modal Body */}
            <div className="p-5 space-y-4">
              <div className="flex items-start gap-4">
                {/* Form Inputs */}
                <div className="flex-1 space-y-3 text-xs">
                  <div className="flex items-center gap-3">
                    <label className="w-16 text-right text-[#98989d] font-medium">Address</label>
                    <input
                      type="text"
                      placeholder="https://..."
                      value={urlInput}
                      onChange={(e) => handleUrlChange(e.target.value)}
                      className="flex-1 bg-[#1c1c1e] border border-[#3e3e42] rounded px-2.5 py-1.5 text-white font-mono text-xs focus:outline-none focus:border-[#007aff]"
                      autoFocus
                    />
                  </div>

                  <div className="flex items-center gap-3">
                    <label className="w-16 text-right text-[#98989d] font-medium">File</label>
                    <input
                      type="text"
                      placeholder="filename.ext"
                      value={filenameInput}
                      onChange={(e) => setFilenameInput(e.target.value)}
                      className="flex-1 bg-[#1c1c1e] border border-[#3e3e42] rounded px-2.5 py-1.5 text-white text-xs focus:outline-none focus:border-[#007aff]"
                    />
                  </div>

                  <div className="flex items-center gap-3">
                    <label className="w-16 text-right text-[#98989d] font-medium">Save in</label>
                    <div className="flex-1 flex items-center gap-1.5">
                      <input
                        type="text"
                        value={saveInInput}
                        onChange={(e) => setSaveInInput(e.target.value)}
                        className="flex-1 bg-[#1c1c1e] border border-[#3e3e42] rounded px-2.5 py-1.5 text-white text-xs focus:outline-none focus:border-[#007aff]"
                      />
                      <button className="p-1.5 bg-[#3a3a3c] rounded text-[#d1d1d6] hover:text-white">
                        <Folder className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  <div className="flex justify-between items-center pl-19 pt-1 text-[11px] text-[#8e8e93]">
                    <div className="flex items-center gap-3">
                      <span>Free space 24.9 GB</span>
                      {probeLoading ? (
                        <span className="flex items-center gap-1.5 text-[#007aff]">
                          <RefreshCw className="w-3 h-3 animate-spin" /> Checking size...
                        </span>
                      ) : probeSize ? (
                        <span className="text-[#e1e1e6]">File size: {formatBytes(probeSize)}</span>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-1">
                      <span>Threads:</span>
                      <select
                        value={threads}
                        onChange={(e) => setThreads(Number(e.target.value))}
                        className="bg-[#1c1c1e] border border-[#3e3e42] rounded px-1 py-0.5 text-white"
                      >
                        <option value={4}>4</option>
                        <option value={8}>8</option>
                        <option value={16}>16</option>
                        <option value={32}>32</option>
                      </select>
                    </div>
                  </div>
                </div>

                {/* File Icon Preview */}
                <div className="w-20 h-20 bg-[#212124] border border-[#38383c] rounded-lg flex flex-col items-center justify-center text-[#8e8e93] shrink-0">
                  <File className="w-8 h-8 stroke-[1.2]" />
                  <span className="text-[10px] mt-1 font-mono uppercase truncate max-w-[60px]">
                    {probeSize ? formatBytes(probeSize) : (filenameInput.split('.').pop() || '---')}
                  </span>
                </div>
              </div>

              {/* Bottom Actions (Matching Image 2) */}
              <div className="flex items-center justify-end gap-3 pt-3 border-t border-[#38383c]">
                <button
                  onClick={() => setIsAddOpen(false)}
                  className="px-5 py-1.5 rounded-md text-xs font-medium bg-[#3a3a3c] hover:bg-[#48484a] text-white transition"
                >
                  Cancel
                </button>
                <button
                  onClick={handleAddDownload}
                  className="px-6 py-1.5 rounded-md text-xs font-medium bg-[#007aff] hover:bg-[#006ae6] text-white transition shadow-sm"
                >
                  Download
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Active Download Progress Window (Matching Image 3) */}
      {activeProgressTask && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-[#2d2d30] border border-[#3e3e42] rounded-xl w-full max-w-lg shadow-2xl overflow-hidden font-sans">
            {/* macOS Header */}
            <div className="h-9 bg-[#252528] border-b border-[#38383c] flex items-center justify-between px-3 relative">
              <div className="flex items-center gap-2">
                <button onClick={() => setActiveProgressTask(null)} className="w-3 h-3 rounded-full bg-[#ff5f56] border border-[#e0443e]" />
                <div className="w-3 h-3 rounded-full bg-[#ffbd2e] border border-[#dea123]" />
                <div className="w-3 h-3 rounded-full bg-[#27c93f] border border-[#1aab29]" />
              </div>
              <h3 className="text-xs font-semibold text-[#e1e1e6] truncate max-w-[320px] absolute left-1/2 -translate-x-1/2">
                [ {activeProgressTask.totalBytes > 0 ? ((activeProgressTask.downloaded / activeProgressTask.totalBytes) * 100).toFixed(0) : 0}% ] {activeProgressTask.filename}
              </h3>
              <button onClick={() => setActiveProgressTask(null)} className="text-[#8e8e93] hover:text-white">
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* Modal Content */}
            <div className="p-6 space-y-6">
              <div className="flex items-center gap-6">
                {/* Dashed Circular Progress Ring */}
                <div className="relative w-24 h-24 shrink-0 flex items-center justify-center">
                  <svg className="w-full h-full transform -rotate-90" viewBox="0 0 36 36">
                    <path
                      className="text-[#3a3a3c]"
                      strokeWidth="2.5"
                      strokeDasharray="2, 2"
                      stroke="currentColor"
                      fill="none"
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                    />
                    <path
                      className="text-[#007aff]"
                      strokeWidth="3"
                      strokeDasharray={`${activeProgressTask.totalBytes > 0 ? (activeProgressTask.downloaded / activeProgressTask.totalBytes) * 100 : 0}, 100`}
                      strokeLinecap="round"
                      stroke="currentColor"
                      fill="none"
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                    />
                  </svg>
                  <span className="absolute font-bold text-sm text-white font-sans">
                    {activeProgressTask.totalBytes > 0 ? ((activeProgressTask.downloaded / activeProgressTask.totalBytes) * 100).toFixed(0) : 0}%
                  </span>
                </div>

                {/* Progress Stats */}
                <div className="flex-1 space-y-2 min-w-0">
                  <h4 className="text-sm font-semibold text-white truncate" title={activeProgressTask.filename}>
                    {activeProgressTask.filename}
                  </h4>
                  <p className="text-xs text-[#8e8e93]">
                    {activeProgressTask.status === 'downloading' ? 'Downloading...' : 'Paused'}
                  </p>

                  <div className="flex justify-between text-xs text-[#d1d1d6] font-medium pt-1">
                    <span>
                      Downloaded {activeProgressTask.totalBytes > 0 ? ((activeProgressTask.downloaded / activeProgressTask.totalBytes) * 100).toFixed(0) : 0}%
                    </span>
                    <span>
                      {formatBytes(activeProgressTask.downloaded)} / {formatBytes(activeProgressTask.totalBytes)}
                    </span>
                  </div>

                  {/* Multi-segment Chunk Bar (IDM / XDM Style Segment Pills) */}
                  <div className="grid grid-cols-8 gap-1 py-1">
                    {(activeProgressTask.chunks && activeProgressTask.chunks.length > 0
                      ? activeProgressTask.chunks
                      : Array.from({ length: 8 }).map((_, i) => ({ idx: i, startOffset: 0, endOffset: 100, downloaded: activeProgressTask.status === 'completed' ? 100 : 50, done: false }))
                    ).map((c) => {
                      const segTotal = c.endOffset - c.startOffset;
                      const segPct = segTotal > 0 ? (c.downloaded / segTotal) * 100 : 0;
                      return (
                        <div key={c.idx} className="bg-[#3a3a3c] h-2 rounded overflow-hidden">
                          <div
                            className="bg-[#007aff] h-full transition-all duration-200"
                            style={{ width: `${segPct}%` }}
                          />
                        </div>
                      );
                    })}
                  </div>

                  <div className="flex justify-between text-xs text-[#8e8e93] pt-0.5 font-mono">
                    <span>Speed {formatSpeed(activeProgressTask.speed || 0)}</span>
                    <span>
                      {formatETA(
                        activeProgressTask.speed && activeProgressTask.speed > 0
                          ? (activeProgressTask.totalBytes - activeProgressTask.downloaded) / activeProgressTask.speed
                          : 0
                      )}
                    </span>
                  </div>
                </div>
              </div>

              {/* Bottom Actions (Matching Image 3) */}
              <div className="flex items-center justify-end gap-3 pt-3 border-t border-[#38383c]">
                <button
                  onClick={() => setActiveProgressTask(null)}
                  className="px-6 py-1.5 rounded-md text-xs font-medium bg-[#3a3a3c] hover:bg-[#48484a] text-white transition"
                >
                  Hide
                </button>
                <button
                  onClick={() => togglePause(activeProgressTask)}
                  className="px-6 py-1.5 rounded-md text-xs font-medium bg-[#007aff] hover:bg-[#006ae6] text-white transition shadow-sm"
                >
                  {activeProgressTask.status === 'downloading' ? 'Pause' : 'Resume'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
