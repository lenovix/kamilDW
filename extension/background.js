const SERVER_URL = "http://127.0.0.1:19890/api/download";

// Track detected media per tab
const tabMediaMap = new Map();

// 1. Context Menu
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "kamildw-download",
    title: "Download with kamilDW",
    contexts: ["link", "video", "audio", "image"]
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  const url = info.linkUrl || info.srcUrl;
  if (url) {
    sendToKamilDW(url);
  }
});

// 2. Intercept Downloads
chrome.downloads.onCreated.addListener((downloadItem) => {
  if (downloadItem.url.startsWith("http://127.0.0.1") || downloadItem.url.startsWith("blob:")) {
    return;
  }
  sendToKamilDW(downloadItem.url, downloadItem.filename).then((ok) => {
    if (ok) {
      chrome.downloads.cancel(downloadItem.id);
    }
  });
});

// 3. Sniff video/audio network requests (YouTube, TikTok, Facebook, generic streaming)
chrome.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.tabId < 0) return;
    const url = details.url;

    // Detect YouTube videoplayback, HLS (.m3u8), DASH (.mpd), and direct video files
    const isYT = url.includes("googlevideo.com/videoplayback");
    const isManifest = url.includes(".m3u8") || url.includes(".mpd");
    const isDirect = /\.(mp4|webm|mkv|mov|ts)(\?.*)?$/i.test(url);

    if (isYT || isManifest || isDirect) {
      let list = tabMediaMap.get(details.tabId) || [];
      if (!list.some((item) => item.url === url)) {
        let label = "Video Stream";
        if (isYT) label = "YouTube Video Stream";
        else if (url.includes(".m3u8")) label = "HLS Stream (.m3u8)";
        else if (isDirect) label = "Direct Video";

        list.push({ url, label, type: details.type });
        tabMediaMap.set(details.tabId, list);

        // Notify content script in tab
        chrome.tabs.sendMessage(details.tabId, {
          type: "KAMILDW_MEDIA_FOUND",
          media: list
        }).catch(() => {});
      }
    }
  },
  { urls: ["<all_urls>"] }
);

// Clean up tab data on close
chrome.tabs.onRemoved.addListener((tabId) => {
  tabMediaMap.delete(tabId);
});

// 4. Message relay from content script
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === "KAMILDW_GET_MEDIA" && sender.tab) {
    const list = tabMediaMap.get(sender.tab.id) || [];
    sendResponse({ media: list });
  } else if (msg.type === "KAMILDW_DOWNLOAD") {
    sendToKamilDW(msg.url, msg.filename).then((ok) => sendResponse({ ok }));
    return true;
  }
});

async function sendToKamilDW(url, filename = "") {
  try {
    const resp = await fetch(SERVER_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        url,
        filename,
        headers: {
          "User-Agent": navigator.userAgent,
          "Referer": "https://www.youtube.com/"
        }
      })
    });
    return resp.ok;
  } catch (err) {
    console.warn("[kamilDW] Engine not connected:", err);
    return false;
  }
}
