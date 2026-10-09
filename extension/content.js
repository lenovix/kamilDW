(() => {
  let floatingBtn = null;
  let mediaList = [];
  let modal = null;

  function isExtensionValid() {
    return typeof chrome !== "undefined" && chrome.runtime && !!chrome.runtime.id;
  }

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  }

  function safeSendMessage(message, callback) {
    if (!isExtensionValid()) {
      console.warn("[kamilDW] Extension context invalidated. Please refresh the page.");
      return;
    }
    try {
      chrome.runtime.sendMessage(message, (response) => {
        const lastErr = chrome.runtime.lastError;
        if (lastErr) {
          return;
        }
        if (callback) callback(response);
      });
    } catch (e) {
      console.warn("[kamilDW] SendMessage error:", e);
    }
  }

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
      display: none; align-items: center; gap: 6px; user-select: none;
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
      openNewDownloadModal();
    };

    return floatingBtn;
  }

  async function probeUrl(url, onResult) {
    try {
      const resp = await fetch("http://127.0.0.1:19890/api/probe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url })
      });
      if (resp.ok) {
        const data = await resp.json();
        onResult(data);
      }
    } catch {
      // Offline / probe failed
    }
  }

  function openNewDownloadModal() {
    if (modal) return;

    const initialUrl = mediaList.length > 0 ? mediaList[0].url : '';
    const initialFilename = document.title ? `${document.title.replace(/[/\\?%*:|"<>]/g, '')}.mp4` : "video.mp4";

    modal = document.createElement("div");
    modal.id = "kamildw-new-download-modal";
    modal.style.cssText = `
      position: fixed; top: 0; left: 0; width: 100%; height: 100%;
      background: rgba(0,0,0,0.5); backdrop-filter: blur(2px);
      display: flex; align-items: center; justify-content: center; z-index: 2147483647;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    `;

    modal.innerHTML = `
      <div style="
        background: #2d2d30; border: 1px solid #3e3e42; border-radius: 12px;
        width: 520px; max-width: 90vw; box-shadow: 0 20px 50px rgba(0,0,0,0.5);
        overflow: hidden; color: #d1d1d6;
      ">
        <!-- macOS Header -->
        <div style="
          height: 36px; background: #252528; border-bottom: 1px solid #38383c;
          display: flex; align-items: center; justify-content: space-between; padding: 0 12px; position: relative;
        ">
          <div style="display: flex; gap: 8px;">
            <button id="kamildw-modal-close" style="
              width: 12px; height: 12px; border-radius: 50%; background: #ff5f56;
              border: 1px solid #e0443e; cursor: pointer; padding: 0;
            "></button>
            <div style="width: 12px; height: 12px; border-radius: 50%; background: #ffbd2e; border: 1px solid #dea123;"></div>
            <div style="width: 12px; height: 12px; border-radius: 50%; background: #27c93f; border: 1px solid #1aab29;"></div>
          </div>
          <h3 style="
            font-size: 13px; font-weight: 600; color: #e1e1e6; position: absolute;
            left: 50%; transform: translateX(-50%); margin: 0;
          ">New download</h3>
          <div style="width: 36px;"></div>
        </div>

        <!-- Body -->
        <div style="padding: 20px; display: flex; gap: 16px;">
          <div style="flex: 1; display: flex; flex-direction: column; gap: 12px;">
            <div style="display: flex; align-items: center; gap: 12px;">
              <label style="width: 64px; text-align: right; font-size: 12px; color: #98989d; font-weight: 500;">Address</label>
              <input type="text" id="kamildw-url" value="${initialUrl}" placeholder="https://..." style="
                flex: 1; background: #1c1c1e; border: 1px solid #3e3e42; border-radius: 4px;
                padding: 6px 10px; color: #fff; font-family: monospace; font-size: 12px; outline: none;
              " />
            </div>

            <div style="display: flex; align-items: center; gap: 12px;">
              <label style="width: 64px; text-align: right; font-size: 12px; color: #98989d; font-weight: 500;">File</label>
              <input type="text" id="kamildw-filename" value="${initialFilename}" placeholder="filename.ext" style="
                flex: 1; background: #1c1c1e; border: 1px solid #3e3e42; border-radius: 4px;
                padding: 6px 10px; color: #fff; font-size: 12px; outline: none;
              " />
            </div>

            <div style="display: flex; align-items: center; gap: 12px;">
              <label style="width: 64px; text-align: right; font-size: 12px; color: #98989d; font-weight: 500;">Save in</label>
              <div style="flex: 1; display: flex; gap: 6px;">
                <input type="text" id="kamildw-savein" value="/Downloads" style="
                  flex: 1; background: #1c1c1e; border: 1px solid #3e3e42; border-radius: 4px;
                  padding: 6px 10px; color: #fff; font-size: 12px; outline: none;
                " />
                <button style="
                  background: #3a3a3c; border: none; border-radius: 4px; padding: 6px 8px;
                  color: #d1d1d6; cursor: pointer; display: flex; align-items: center;
                ">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
                </button>
              </div>
            </div>

            <div style="padding-left: 76px; font-size: 11px; color: #8e8e93; margin-top: -4px; display: flex; justify-content: space-between;">
              <span>Free space 24.9 GB</span>
              <span id="kamildw-size-info" style="color: #007aff; font-weight: 500;">Checking size...</span>
            </div>
          </div>

          <!-- File Icon Preview -->
          <div style="
            width: 80px; height: 80px; background: #212124; border: 1px solid #38383c;
            border-radius: 8px; display: flex; flex-direction: column; align-items: center;
            justify-content: center; color: #8e8e93; flex-shrink: 0;
          ">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
            <span id="kamildw-size-badge" style="font-size: 10px; margin-top: 4px; font-family: monospace; text-transform: uppercase;">
              ${initialFilename.split('.').pop() || '---'}
            </span>
          </div>
        </div>

        <!-- Footer Actions -->
        <div style="
          padding: 12px 20px; border-top: 1px solid #38383c; display: flex;
          justify-content: flex-end; gap: 12px;
        ">
          <button id="kamildw-cancel" style="
            padding: 6px 24px; border-radius: 6px; font-size: 12px; font-weight: 500;
            background: #3a3a3c; color: #fff; border: none; cursor: pointer;
          ">Cancel</button>
          <button id="kamildw-start" style="
            padding: 6px 24px; border-radius: 6px; font-size: 12px; font-weight: 500;
            background: #007aff; color: #fff; border: none; cursor: pointer;
          ">Download</button>
        </div>
      </div>
    `;

    document.body.appendChild(modal);

    const sizeInfoEl = modal.querySelector("#kamildw-size-info");
    const sizeBadgeEl = modal.querySelector("#kamildw-size-badge");
    const urlInputEl = modal.querySelector("#kamildw-url");
    const filenameInputEl = modal.querySelector("#kamildw-filename");
    const saveInEl = modal.querySelector("#kamildw-savein");

    const doProbe = (url) => {
      if (!url) {
        if (sizeInfoEl) sizeInfoEl.innerText = "";
        return;
      }
      if (sizeInfoEl) sizeInfoEl.innerText = "Checking size...";
      probeUrl(url, (data) => {
        if (data.total > 0) {
          const sz = formatBytes(data.total);
          if (sizeInfoEl) sizeInfoEl.innerText = `File size: ${sz}`;
          if (sizeBadgeEl) sizeBadgeEl.innerText = sz;
        } else {
          if (sizeInfoEl) sizeInfoEl.innerText = "";
        }
        if (data.filename && data.filename !== "download.bin" && data.filename !== "video.mp4") {
          if (filenameInputEl && !filenameInputEl.value.endsWith('.mp4')) {
            filenameInputEl.value = data.filename;
          }
        }
      });
    };

    if (initialUrl) {
      doProbe(initialUrl);
    }

    urlInputEl.oninput = (e) => {
      doProbe(e.target.value);
    };

    // Event Listeners
    modal.querySelector("#kamildw-modal-close").onclick = closeModal;
    modal.querySelector("#kamildw-cancel").onclick = closeModal;
    modal.onclick = (e) => {
      if (e.target === modal) closeModal();
    };

    modal.querySelector("#kamildw-start").onclick = () => {
      const url = urlInputEl.value;
      const filename = filenameInputEl.value;
      const savePath = saveInEl ? saveInEl.value : "";

      if (url) {
        safeSendMessage({
          type: "KAMILDW_DOWNLOAD",
          url: url,
          filename: filename,
          savePath: savePath
        });
        closeModal();
      }
    };
  }

  function closeModal() {
    if (modal) {
      modal.remove();
      modal = null;
    }
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
  if (isExtensionValid()) {
    try {
      chrome.runtime.onMessage.addListener((msg) => {
        if (msg.type === "KAMILDW_MEDIA_FOUND") {
          mediaList = msg.media;
          updateWidgetVisibility();
        }
      });
    } catch (e) {
      console.warn("[kamilDW] Listener error:", e);
    }
  }

  // Initial check
  safeSendMessage({ type: "KAMILDW_GET_MEDIA" }, (resp) => {
    if (resp && resp.media) {
      mediaList = resp.media;
      updateWidgetVisibility();
    }
  });

  // Re-attach on DOM dynamic changes
  const observer = new MutationObserver(() => {
    if (!isExtensionValid()) {
      observer.disconnect();
      return;
    }
    updateWidgetVisibility();
  });
  observer.observe(document.body, { childList: true, subtree: true });
})();
