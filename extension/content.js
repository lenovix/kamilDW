(() => {
  let floatingBtn = null;
  let mediaList = [];

  function createFloatingButton() {
    if (floatingBtn) return floatingBtn;

    floatingBtn = document.createElement("div");
    floatingBtn.id = "kamildw-floating-bar";
    floatingBtn.style.cssText = `
      position: absolute; top: 12px; right: 12px; z-index: 2147483647;
      background: linear-gradient(135deg, #0ea5e9, #6366f1);
      color: #ffffff; border-radius: 8px; padding: 6px 12px;
      font-family: system-ui, -apple-system, sans-serif; font-size: 12px;
      font-weight: 600; cursor: pointer; box-shadow: 0 4px 14px rgba(0,0,0,0.35);
      display: none; items-center: center; gap: 6px; user-select: none;
      border: 1px solid rgba(255,255,255,0.2); transition: transform 0.15s ease;
    `;

    floatingBtn.innerHTML = `
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
      <span>Download ini dengan kamilDW</span>
    `;

    floatingBtn.onmouseover = () => { floatingBtn.style.transform = "scale(1.03)"; };
    floatingBtn.onmouseout = () => { floatingBtn.style.transform = "scale(1)"; };
    floatingBtn.onclick = (e) => {
      e.stopPropagation();
      e.preventDefault();
      if (mediaList.length > 0) {
        const item = mediaList[0];
        chrome.runtime.sendMessage({
          type: "KAMILDW_DOWNLOAD",
          url: item.url,
          filename: document.title ? `${document.title.replace(/[/\\?%*:|"<>]/g, '')}.mp4` : "video.mp4"
        });
        floatingBtn.innerText = "✓ Dikirim ke kamilDW";
        setTimeout(() => {
          floatingBtn.innerHTML = `
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
            <span>Download ini dengan kamilDW</span>
          `;
        }, 2000);
      }
    };

    return floatingBtn;
  }

  function attachToVideoPlayer() {
    const video = document.querySelector("video");
    if (!video) return;

    const btn = createFloatingButton();
    const container = video.parentElement;

    if (container && getComputedStyle(container).position === "static") {
      container.style.position = "relative";
    }

    if (container && !container.contains(btn)) {
      container.appendChild(btn);
    }
  }

  function updateWidgetVisibility() {
    attachToVideoPlayer();
    if (floatingBtn) {
      if (mediaList.length > 0) {
        floatingBtn.style.display = "flex";
      } else {
        floatingBtn.style.display = "none";
      }
    }
  }

  // Listen for media detected from background service worker
  chrome.runtime.onMessage.addListener((msg) => {
    if (msg.type === "KAMILDW_MEDIA_FOUND") {
      mediaList = msg.media;
      updateWidgetVisibility();
    }
  });

  // Initial check
  chrome.runtime.sendMessage({ type: "KAMILDW_GET_MEDIA" }, (resp) => {
    if (resp && resp.media) {
      mediaList = resp.media;
      updateWidgetVisibility();
    }
  });

  // Re-attach on DOM dynamic changes
  const observer = new MutationObserver(() => {
    updateWidgetVisibility();
  });
  observer.observe(document.body, { childList: true, subtree: true });
})();
