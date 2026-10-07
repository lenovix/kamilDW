(() => {
  const SERVER_URL = "http://127.0.0.1:19890/api/download";
  const MEDIA_EXT = /\.(mp4|webm|mkv|avi|mov|ts|m4v|mp3|flac|wav|aac|ogg|m3u8|mpd)(\?.*)?$/i;

  let widget = null;

  function createWidget() {
    if (widget) return widget;

    widget = document.createElement("div");
    widget.id = "kamildw-widget";
    widget.style.cssText = `
      position: fixed; bottom: 24px; right: 24px; z-index: 2147483647;
      background: #0f172a; color: #e2e8f0; border-radius: 12px;
      padding: 12px 16px; box-shadow: 0 8px 32px rgba(0,0,0,0.4);
      font-family: system-ui, sans-serif; font-size: 13px;
      display: none; flex-direction: column; gap: 8px; min-width: 220px;
      border: 1px solid #334155;
    `;

    const title = document.createElement("div");
    title.textContent = "kamilDW";
    title.style.cssText = "font-weight: 600; font-size: 14px; color: #38bdf8;";

    const btn = document.createElement("button");
    btn.textContent = "Download Media";
    btn.style.cssText = `
      background: #0ea5e9; color: #fff; border: none; border-radius: 8px;
      padding: 8px 12px; cursor: pointer; font-size: 13px; font-weight: 500;
    `;
    btn.onclick = () => {
      if (widget._url) {
        sendToKamilDW(widget._url, widget._filename);
        widget.style.display = "none";
      }
    };

    widget.appendChild(title);
    widget.appendChild(btn);
    document.body.appendChild(widget);
    return widget;
  }

  function showWidget(url, filename) {
    const w = createWidget();
    w._url = url;
    w._filename = filename || "";
    w.style.display = "flex";
  }

  function hideWidget() {
    if (widget) widget.style.display = "none";
  }

  function sendToKamilDW(url, filename) {
    fetch(SERVER_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url, filename })
    }).catch((err) => console.warn("[kamilDW] Engine not connected:", err));
  }

  // Sniff media from video/audio elements
  function sniffMediaElements() {
    const videos = document.querySelectorAll("video");
    const audios = document.querySelectorAll("audio");
    const sources = [];

    videos.forEach((v) => {
      if (v.src && MEDIA_EXT.test(v.src)) sources.push(v.src);
      v.querySelectorAll("source").forEach((s) => {
        if (s.src && MEDIA_EXT.test(s.src)) sources.push(s.src);
      });
    });
    audios.forEach((a) => {
      if (a.src && MEDIA_EXT.test(a.src)) sources.push(a.src);
    });

    return [...new Set(sources)];
  }

  // Sniff HLS manifests from network (via performance entries)
  function sniffHLS() {
    const entries = performance.getEntriesByType("resource");
    return entries
      .map((e) => e.name)
      .filter((u) => /\.m3u8|\.mpd/i.test(u));
  }

  // Sniff media links in DOM
  function sniffLinks() {
    const links = document.querySelectorAll("a[href]");
    const urls = [];
    links.forEach((a) => {
      if (MEDIA_EXT.test(a.href)) urls.push(a.href);
    });
    return urls;
  }

  function scan() {
    const mediaUrls = [...sniffMediaElements(), ...sniffHLS(), ...sniffLinks()];
    if (mediaUrls.length > 0) {
      showWidget(mediaUrls[0], "");
    } else {
      hideWidget();
    }
  }

  // Observe DOM for dynamically loaded media
  const observer = new MutationObserver(() => {
    clearTimeout(window._kamildwScanTimer);
    window._kamildwScanTimer = setTimeout(scan, 1500);
  });
  observer.observe(document.body, { childList: true, subtree: true });

  // Initial scan
  if (document.readyState === "complete") {
    scan();
  } else {
    window.addEventListener("load", scan);
  }

  // Re-scan on navigation (SPA)
  let lastUrl = location.href;
  setInterval(() => {
    if (location.href !== lastUrl) {
      lastUrl = location.href;
      setTimeout(scan, 1000);
    }
  }, 2000);
})();
