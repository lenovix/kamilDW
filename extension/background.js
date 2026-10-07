const SERVER_URL = "http://127.0.0.1:19890/api/download";

// 1. Context Menu Registration
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

// 2. Intercept Chrome Downloads
chrome.downloads.onCreated.addListener((downloadItem) => {
  if (downloadItem.url.startsWith("http://127.0.0.1") || downloadItem.url.startsWith("blob:")) {
    return;
  }
  
  // Send to desktop engine
  sendToKamilDW(downloadItem.url, downloadItem.filename).then((ok) => {
    if (ok) {
      chrome.downloads.cancel(downloadItem.id);
    }
  });
});

async function sendToKamilDW(url, filename = "") {
  try {
    const resp = await fetch(SERVER_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url, filename })
    });
    return resp.ok;
  } catch (err) {
    console.warn("[kamilDW] Engine not connected:", err);
    return false;
  }
}
