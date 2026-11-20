(function () {
  "use strict";

  const PREFS_STORAGE_KEY = "lvp:prefs";
  const PROGRESS_KEY_PREFIX = "lvp:progress:";

  // Application State
  const state = {
    roots: [],
    browseBase: { enabled: true, label: "Diretório Base" },
    activeRootId: "",
    currentPath: "",
    breadcrumbs: [],
    folders: [],
    videos: [],
    searchQuery: "",
    isSearching: false,
    prefs: loadPreferences(),
    // Player state
    currentVideoIndex: -1,
    currentVideo: null,
    triedTranscodeFallback: false,
    isTranscodedStream: false,
    transcodeBaseOffset: 0,
    totalDuration: 0,
    isDraggingSeek: false,
    fsHideTimer: null,
    autoplayTimer: null,
    autoplayRemaining: 5,
    lastProgressSaveAt: 0,
    // Folder Picker Modal state
    pickerCurrentPath: "",
    pickerSelectedPaths: new Set(),
    // In-memory thumbnail cache for client-side fallback
    thumbCache: new Map(),
  };

  // DOM Elements
  const dom = {
    brandHome: document.getElementById("brandHome"),
    searchInput: document.getElementById("searchInput"),
    clearSearchBtn: document.getElementById("clearSearchBtn"),
    sortSelect: document.getElementById("sortSelect"),
    openLibraryModalBtn: document.getElementById("openLibraryModalBtn"),
    rootsBarSection: document.getElementById("rootsBarSection"),
    rootsTabs: document.getElementById("rootsTabs"),
    breadcrumbsNav: document.getElementById("breadcrumbsNav"),
    breadcrumbsList: document.getElementById("breadcrumbsList"),
    alertBanner: document.getElementById("alertBanner"),
    alertMessage: document.getElementById("alertMessage"),
    alertRetryBtn: document.getElementById("alertRetryBtn"),
    foldersSection: document.getElementById("foldersSection"),
    foldersCount: document.getElementById("foldersCount"),
    foldersGrid: document.getElementById("foldersGrid"),
    videosSection: document.getElementById("videosSection"),
    videosSectionTitle: document.getElementById("videosSectionTitle"),
    videosCount: document.getElementById("videosCount"),
    videosGrid: document.getElementById("videosGrid"),
    emptyState: document.getElementById("emptyState"),
    emptyTitle: document.getElementById("emptyTitle"),
    emptyDescription: document.getElementById("emptyDescription"),
    emptyAddFoldersBtn: document.getElementById("emptyAddFoldersBtn"),
    emptyResetSearchBtn: document.getElementById("emptyResetSearchBtn"),
    appVersionText: document.getElementById("appVersionText"),

    // Player Modal
    playerModal: document.getElementById("playerModal"),
    playerDialog: document.querySelector("#playerModal .player-dialog"),
    playerBottomControls: document.getElementById("playerBottomControls"),
    playerVideoTitle: document.getElementById("playerVideoTitle"),
    playerVideoPath: document.getElementById("playerVideoPath"),
    prevVideoBtn: document.getElementById("prevVideoBtn"),
    nextVideoBtn: document.getElementById("nextVideoBtn"),
    closePlayerBtn: document.getElementById("closePlayerBtn"),
    resumeBanner: document.getElementById("resumeBanner"),
    resumeBannerText: document.getElementById("resumeBannerText"),
    restartFromBeginningBtn: document.getElementById("restartFromBeginningBtn"),
    playerErrorBanner: document.getElementById("playerErrorBanner"),
    errorBackToLibBtn: document.getElementById("errorBackToLibBtn"),
    videoElement: document.getElementById("videoElement"),
    autoplayOverlay: document.getElementById("autoplayOverlay"),
    autoplayCountdown: document.getElementById("autoplayCountdown"),
    autoplayNextTitle: document.getElementById("autoplayNextTitle"),
    autoplayPlayNowBtn: document.getElementById("autoplayPlayNowBtn"),
    autoplayCancelBtn: document.getElementById("autoplayCancelBtn"),
    speedSelect: document.getElementById("speedSelect"),
    subtitleSelect: document.getElementById("subtitleSelect"),
    audioTrackSelect: document.getElementById("audioTrackSelect"),
    autoplayNextCheckbox: document.getElementById("autoplayNextCheckbox"),
    playPauseBtn: document.getElementById("playPauseBtn"),
    seekBackBtn: document.getElementById("seekBackBtn"),
    seekFwdBtn: document.getElementById("seekFwdBtn"),
    currentTimeLabel: document.getElementById("currentTimeLabel"),
    seekSlider: document.getElementById("seekSlider"),
    totalDurationLabel: document.getElementById("totalDurationLabel"),
    muteBtn: document.getElementById("muteBtn"),
    volumeSlider: document.getElementById("volumeSlider"),
    fullscreenBtn: document.getElementById("fullscreenBtn"),

    // Library Folder Picker Modal
    libraryModal: document.getElementById("libraryModal"),
    closeLibraryModalBtn: document.getElementById("closeLibraryModalBtn"),
    activeRootsList: document.getElementById("activeRootsList"),
    pickerSection: document.getElementById("pickerSection"),
    selectCurrentDirBtn: document.getElementById("selectCurrentDirBtn"),
    pickerBreadcrumbsList: document.getElementById("pickerBreadcrumbsList"),
    pickerStatusMsg: document.getElementById("pickerStatusMsg"),
    pickerDirsList: document.getElementById("pickerDirsList"),
    pickerSelectedCount: document.getElementById("pickerSelectedCount"),
    addSelectedDirsBtn: document.getElementById("addSelectedDirsBtn"),
  };

  // --- Preferences & Playback Progress (data-model.md 2.1 & 2.2) ---

  function loadPreferences() {
    const defaults = {
      playbackRate: 1.0,
      autoPlayNext: true,
      sortBy: "name_asc",
      volume: 1.0,
      muted: false,
    };
    try {
      const raw = localStorage.getItem(PREFS_STORAGE_KEY);
      if (!raw) return defaults;
      const parsed = JSON.parse(raw);
      return {
        playbackRate: Number(parsed.playbackRate) || 1.0,
        autoPlayNext: typeof parsed.autoPlayNext === "boolean" ? parsed.autoPlayNext : true,
        sortBy: parsed.sortBy || "name_asc",
        volume: typeof parsed.volume === "number" ? Math.min(1, Math.max(0, parsed.volume)) : 1.0,
        muted: Boolean(parsed.muted),
      };
    } catch (err) {
      console.debug("Falha ao ler preferências:", err);
      return defaults;
    }
  }

  function savePreferences() {
    try {
      localStorage.setItem(PREFS_STORAGE_KEY, JSON.stringify(state.prefs));
    } catch (err) {
      console.debug("Armazenamento local indisponível:", err);
    }
  }

  function getPlaybackProgress(videoId) {
    if (!videoId) return null;
    try {
      const raw = localStorage.getItem(PROGRESS_KEY_PREFIX + videoId);
      if (!raw) return null;
      return JSON.parse(raw);
    } catch (err) {
      console.debug("Falha ao ler progresso:", err);
      return null;
    }
  }

  function savePlaybackProgress(videoId, currentTime, duration, forceCompleted) {
    if (!videoId || !duration || !Number.isFinite(duration) || duration <= 0) return;
    const ratio = Math.min(1, Math.max(0, currentTime / duration));
    const completed = Boolean(forceCompleted || ratio >= 0.95);
    const entry = {
      videoId: videoId,
      currentTime: completed ? 0 : currentTime,
      duration: duration,
      progressRatio: completed ? 1.0 : ratio,
      completed: completed,
      updatedAt: Date.now(),
    };
    try {
      localStorage.setItem(PROGRESS_KEY_PREFIX + videoId, JSON.stringify(entry));
    } catch (err) {
      console.debug("Falha ao salvar progresso:", err);
    }
  }

  // --- Formatting Helpers ---

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    const val = bytes / Math.pow(1024, i);
    return (i === 0 ? val.toFixed(0) : val.toFixed(1)) + " " + units[i];
  }

  function formatDate(isoStr) {
    if (!isoStr) return "";
    try {
      const d = new Date(isoStr);
      return d.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit", year: "numeric" });
    } catch (err) {
      console.debug("Data inválida:", err);
      return "";
    }
  }

  function formatDuration(sec) {
    if (!Number.isFinite(sec) || sec < 0) return "00:00";
    const total = Math.floor(sec);
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    const mm = String(m).padStart(2, "0");
    const ss = String(s).padStart(2, "0");
    return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
  }

  function showAlert(msg) {
    if (!msg) {
      dom.alertBanner.classList.add("hidden");
      return;
    }
    dom.alertMessage.textContent = msg;
    dom.alertBanner.classList.remove("hidden");
  }

  // --- API Calls ---

  async function apiGet(url) {
    const resp = await fetch(url, { headers: { Accept: "application/json" } });
    if (!resp.ok) {
      let errMsg = `Erro HTTP ${resp.status}`;
      try {
        const data = await resp.json();
        if (data?.error?.message) {
          errMsg = data.error.message;
        }
      } catch (err) {
        console.debug("Resposta de erro não-JSON:", err);
      }
      throw new Error(errMsg);
    }
    return resp.json();
  }

  async function loadRootsAndRefresh(preferredRootId) {
    showAlert("");
    try {
      const data = await apiGet("/api/v1/roots");
      state.roots = data.roots || [];
      if (data.browseBase) {
        state.browseBase = data.browseBase;
      }
      if (data.meta?.version) {
        dom.appVersionText.textContent = "v" + data.meta.version;
      }

      if (state.roots.length === 0) {
        state.activeRootId = "";
        state.currentPath = "";
        state.folders = [];
        state.videos = [];
        renderRootsTabs();
        renderBreadcrumbs([]);
        renderCatalog();
        return;
      }

      const targetRoot =
        state.roots.find((r) => r.id === preferredRootId) ||
        state.roots.find((r) => r.id === state.activeRootId) ||
        state.roots[0];

      state.activeRootId = targetRoot.id;
      renderRootsTabs();

      if (state.isSearching && state.searchQuery) {
        await executeSearch(state.searchQuery);
      } else {
        await browseFolder(state.activeRootId, state.currentPath);
      }
    } catch (err) {
      showAlert(err.message || "Não foi possível carregar as pastas da biblioteca.");
    }
  }

  async function browseFolder(rootId, relPath) {
    showAlert("");
    state.isSearching = false;
    try {
      const params = new URLSearchParams({
        root: rootId || state.activeRootId,
        path: relPath || "",
        sort: state.prefs.sortBy,
      });
      const res = await apiGet("/api/v1/browse?" + params.toString());
      state.activeRootId = res.root.id;
      state.currentPath = res.currentPath || "";
      state.breadcrumbs = res.breadcrumbs || [];
      state.folders = res.folders || [];
      state.videos = res.videos || [];

      renderRootsTabs();
      renderBreadcrumbs(state.breadcrumbs);
      renderCatalog();
    } catch (err) {
      showAlert(err.message || "Falha ao abrir a pasta selecionada.");
    }
  }

  async function executeSearch(query) {
    const trimmed = (query || "").trim();
    if (!trimmed) {
      state.searchQuery = "";
      state.isSearching = false;
      dom.clearSearchBtn.classList.add("hidden");
      if (state.activeRootId) {
        await browseFolder(state.activeRootId, state.currentPath);
      } else {
        renderCatalog();
      }
      return;
    }

    showAlert("");
    state.searchQuery = trimmed;
    state.isSearching = true;
    dom.clearSearchBtn.classList.remove("hidden");

    try {
      const params = new URLSearchParams({
        q: trimmed,
        sort: state.prefs.sortBy,
      });
      const res = await apiGet("/api/v1/search?" + params.toString());
      state.folders = [];
      state.videos = res.videos || [];
      renderBreadcrumbs([
        { label: "Busca na Biblioteca", rootId: state.activeRootId, relPath: state.currentPath },
        { label: `"${trimmed}" (${res.total || 0})`, rootId: "", relPath: "" },
      ]);
      renderCatalog();
    } catch (err) {
      showAlert(err.message || "Falha ao realizar a busca.");
    }
  }

  // --- Rendering ---

  function renderRootsTabs() {
    dom.rootsTabs.innerHTML = "";
    if (state.roots.length === 0) {
      dom.rootsBarSection.classList.add("hidden");
      return;
    }
    dom.rootsBarSection.classList.remove("hidden");

    state.roots.forEach((root) => {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "root-tab" + (root.id === state.activeRootId && !state.isSearching ? " active" : "");
      btn.textContent = root.label;
      btn.addEventListener("click", () => {
        dom.searchInput.value = "";
        state.searchQuery = "";
        state.isSearching = false;
        dom.clearSearchBtn.classList.add("hidden");
        browseFolder(root.id, "");
      });
      dom.rootsTabs.appendChild(btn);
    });
  }

  function renderBreadcrumbs(crumbs) {
    dom.breadcrumbsList.innerHTML = "";
    if (!crumbs || crumbs.length === 0) {
      dom.breadcrumbsNav.classList.add("hidden");
      return;
    }
    dom.breadcrumbsNav.classList.remove("hidden");

    crumbs.forEach((item, idx) => {
      const li = document.createElement("li");
      li.className = "breadcrumb-item";
      const isLast = idx === crumbs.length - 1;

      if (isLast && !state.isSearching) {
        const span = document.createElement("span");
        span.className = "breadcrumb-current";
        span.textContent = item.label;
        li.appendChild(span);
      } else {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "breadcrumb-btn";
        btn.textContent = item.label;
        btn.addEventListener("click", () => {
          dom.searchInput.value = "";
          state.searchQuery = "";
          state.isSearching = false;
          dom.clearSearchBtn.classList.add("hidden");
          browseFolder(item.rootId || state.activeRootId, item.relPath || "");
        });
        li.appendChild(btn);
      }
      dom.breadcrumbsList.appendChild(li);
    });
  }

  function renderCatalog() {
    // Folders
    dom.foldersGrid.innerHTML = "";
    if (!state.isSearching && state.folders.length > 0) {
      dom.foldersSection.classList.remove("hidden");
      dom.foldersCount.textContent = String(state.folders.length);
      state.folders.forEach((folder) => {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "folder-card";
        btn.innerHTML = `
          <svg class="folder-icon" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>
          </svg>
          <div class="folder-meta">
            <span class="folder-name"></span>
            <span class="folder-date"></span>
          </div>
        `;
        btn.querySelector(".folder-name").textContent = folder.name;
        btn.querySelector(".folder-date").textContent = formatDate(folder.modifiedAt);
        btn.addEventListener("click", () => browseFolder(folder.rootId, folder.relPath));
        dom.foldersGrid.appendChild(btn);
      });
    } else {
      dom.foldersSection.classList.add("hidden");
    }

    // Videos
    dom.videosGrid.innerHTML = "";
    if (state.videos.length > 0) {
      dom.videosSection.classList.remove("hidden");
      dom.videosSectionTitle.firstChild.textContent = state.isSearching
        ? "Resultados da Busca "
        : "Vídeos ";
      dom.videosCount.textContent = String(state.videos.length);

      state.videos.forEach((video, index) => {
        const card = createVideoCard(video, index);
        dom.videosGrid.appendChild(card);
      });
    } else {
      dom.videosSection.classList.add("hidden");
    }

    // Empty state handling
    const hasNothing =
      (state.isSearching || state.folders.length === 0) && state.videos.length === 0;
    if (hasNothing) {
      dom.emptyState.classList.remove("hidden");
      if (state.roots.length === 0) {
        dom.emptyTitle.textContent = "Nenhuma pasta adicionada à biblioteca";
        dom.emptyDescription.textContent =
          "Clique em 'Adicionar Pastas à Biblioteca' para selecionar uma ou mais pastas de vídeos.";
        dom.emptyAddFoldersBtn.classList.remove("hidden");
        dom.emptyResetSearchBtn.classList.add("hidden");
      } else if (state.isSearching) {
        dom.emptyTitle.textContent = "Nenhum vídeo encontrado para \"" + state.searchQuery + "\"";
        dom.emptyDescription.textContent =
          "Verifique o termo digitado ou limpe o filtro para voltar à navegação por pastas.";
        dom.emptyAddFoldersBtn.classList.add("hidden");
        dom.emptyResetSearchBtn.classList.remove("hidden");
      } else {
        dom.emptyTitle.textContent = "Esta pasta não contém vídeos compatíveis";
        dom.emptyDescription.textContent =
          "Adicione vídeos (.mp4, .webm, .mkv, .mov, .avi, .ogv) ou selecione outras pastas.";
        dom.emptyAddFoldersBtn.classList.remove("hidden");
        dom.emptyResetSearchBtn.classList.add("hidden");
      }
    } else {
      dom.emptyState.classList.add("hidden");
    }
  }

  function createVideoCard(video, index) {
    const card = document.createElement("button");
    card.type = "button";
    card.className = "video-card";
    card.dataset.videoId = video.id;

    const progress = getPlaybackProgress(video.id);
    const hasSubs = Array.isArray(video.subtitles) && video.subtitles.length > 0;

    card.innerHTML = `
      <div class="video-thumb-wrap">
        <div class="video-thumb-fallback">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M18 4l2 4h-3l-2-4h-2l2 4h-3l-2-4H8l2 4H7L5 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V4h-4z"/>
          </svg>
        </div>
        <div class="video-play-badge">
          <div class="play-circle">
            <svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
              <path fill="currentColor" d="M8 5v14l11-7z"/>
            </svg>
          </div>
        </div>
        <div class="video-badges-top"></div>
      </div>
      <div class="video-info">
        <span class="video-title"></span>
        <span class="video-location hidden"></span>
        <div class="video-meta-row">
          <span class="video-size"></span>
          <span class="video-date"></span>
        </div>
      </div>
    `;

    card.querySelector(".video-title").textContent = video.title || video.name;
    card.querySelector(".video-size").textContent = formatBytes(video.sizeBytes);
    card.querySelector(".video-date").textContent = formatDate(video.modifiedAt);

    if (state.isSearching || state.roots.length > 1) {
      const locEl = card.querySelector(".video-location");
      locEl.textContent = video.folderRelPath
        ? `${video.rootLabel} / ${video.folderRelPath}`
        : video.rootLabel;
      locEl.classList.remove("hidden");
    }

    const badgesTop = card.querySelector(".video-badges-top");
    if (hasSubs) {
      const subBadge = document.createElement("span");
      subBadge.className = "badge-pill badge-sub";
      subBadge.textContent = "CC";
      subBadge.title = `${video.subtitles.length} legenda(s) disponível(is)`;
      badgesTop.appendChild(subBadge);
    }

    const thumbWrap = card.querySelector(".video-thumb-wrap");
    if (progress) {
      if (progress.completed) {
        const doneBadge = document.createElement("span");
        doneBadge.className = "badge-pill badge-done";
        doneBadge.textContent = "Assistido";
        badgesTop.appendChild(doneBadge);
      } else if (progress.progressRatio > 0.01) {
        const track = document.createElement("div");
        track.className = "video-progress-track";
        const fill = document.createElement("div");
        fill.className = "video-progress-fill";
        fill.style.width = `${Math.round(progress.progressRatio * 100)}%`;
        track.appendChild(fill);
        thumbWrap.appendChild(track);
      }
    }

    // Load thumbnail asynchronously without blocking listing (SC-002)
    loadCardThumbnail(video, thumbWrap);

    card.addEventListener("click", () => openPlayer(index));
    return card;
  }

  async function loadCardThumbnail(video, thumbWrap) {
    if (!video.thumbnailUrl) return;

    if (state.thumbCache.has(video.id)) {
      const cachedSrc = state.thumbCache.get(video.id);
      if (cachedSrc) {
        injectThumbImg(thumbWrap, cachedSrc, video.title);
      }
      return;
    }

    try {
      const resp = await fetch(video.thumbnailUrl);
      if (resp.status === 200) {
        const blob = await resp.blob();
        const objectUrl = URL.createObjectURL(blob);
        state.thumbCache.set(video.id, objectUrl);
        injectThumbImg(thumbWrap, objectUrl, video.title);
        return;
      }
    } catch (_) {
      // Fallback to SVG placeholder
    }
    state.thumbCache.set(video.id, null);
  }

  function injectThumbImg(thumbWrap, src, altText) {
    const img = document.createElement("img");
    img.className = "video-thumb-img";
    img.src = src;
    img.alt = altText || "Miniatura";
    img.loading = "lazy";
    const fallback = thumbWrap.querySelector(".video-thumb-fallback");
    if (fallback) {
      fallback.replaceWith(img);
    }
  }

  // --- Video Player Modal (US2 & US4) ---

  function getEffectiveCurrentTime() {
    const v = dom.videoElement;
    return state.transcodeBaseOffset + (Number(v.currentTime) || 0);
  }

  function getEffectiveDuration() {
    const v = dom.videoElement;
    if (state.totalDuration > 0 && Number.isFinite(state.totalDuration)) {
      return state.totalDuration;
    }
    if (v.duration > 0 && Number.isFinite(v.duration)) {
      return v.duration;
    }
    return 0;
  }

  function updateSeekBarUI() {
    const cur = getEffectiveCurrentTime();
    const dur = getEffectiveDuration();
    dom.currentTimeLabel.textContent = formatDuration(cur);
    dom.totalDurationLabel.textContent = dur > 0 ? formatDuration(dur) : "--:--";
    if (!state.isDraggingSeek) {
      dom.seekSlider.max = dur > 0 ? String(Math.floor(dur)) : "100";
      dom.seekSlider.value = dur > 0 ? String(Math.min(Math.floor(dur), Math.floor(cur))) : "0";
      dom.seekSlider.disabled = dur <= 0;
    }
    dom.playPauseBtn.innerHTML = dom.videoElement.paused ? "&#9654;" : "&#10074;&#10074;";
    dom.muteBtn.innerHTML = dom.videoElement.muted || dom.videoElement.volume === 0 ? "&#128263;" : "&#128266;";
    dom.volumeSlider.value = dom.videoElement.muted ? "0" : String(dom.videoElement.volume);
  }

  function seekToSeconds(targetSec) {
    if (!state.currentVideo) return;
    const dur = getEffectiveDuration();
    const clamped = dur > 0 ? Math.max(0, Math.min(dur - 1, targetSec)) : Math.max(0, targetSec);
    const v = dom.videoElement;

    if (!state.isTranscodedStream && Number.isFinite(v.duration) && v.duration > 0) {
      state.transcodeBaseOffset = 0;
      v.currentTime = clamped;
      updateSeekBarUI();
      return;
    }

    // For transcoded streams (.avi, .rmvb, etc.), request server ffmpeg seek via ?transcode=1&start=<sec>
    state.isTranscodedStream = true;
    state.transcodeBaseOffset = Math.floor(clamped);
    const sep = state.currentVideo.streamUrl.includes("?") ? "&" : "?";
    v.src = `${state.currentVideo.streamUrl}${sep}transcode=1&start=${state.transcodeBaseOffset}`;
    v.load();
    v.playbackRate = state.prefs.playbackRate;
    updateSeekBarUI();
    const playPromise = v.play();
    if (playPromise && typeof playPromise.catch === "function") {
      playPromise.catch(() => {});
    }
  }

  async function openPlayer(index) {
    if (index < 0 || index >= state.videos.length) return;
    cancelAutoplayCountdown();

    const video = state.videos[index];
    state.currentVideo = null;
    state.triedTranscodeFallback = false;
    state.isTranscodedStream = false;
    state.transcodeBaseOffset = 0;
    state.totalDuration = 0;
    state.isDraggingSeek = false;

    const v = dom.videoElement;
    v.pause();
    v.removeAttribute("src");
    v.innerHTML = "";

    state.currentVideoIndex = index;
    state.currentVideo = video;

    dom.playerVideoTitle.textContent = video.title || video.name;
    dom.playerVideoPath.textContent = video.folderRelPath
      ? `${video.rootLabel} / ${video.folderRelPath}`
      : video.rootLabel;

    dom.prevVideoBtn.disabled = index <= 0;
    dom.nextVideoBtn.disabled = index >= state.videos.length - 1;
    dom.playerErrorBanner.classList.add("hidden");
    dom.resumeBanner.classList.add("hidden");
    updateSeekBarUI();

    dom.playerModal.classList.remove("hidden");

    // Probe video duration & transcode requirement from server
    const probeUrl = video.streamUrl.replace("/api/v1/stream/", "/api/v1/probe/");
    try {
      const probe = await apiGet(probeUrl);
      if (state.currentVideo !== video) return;
      if (probe && typeof probe.durationSeconds === "number" && probe.durationSeconds > 0) {
        state.totalDuration = probe.durationSeconds;
      }
      if (probe?.requiresTranscode) {
        state.isTranscodedStream = true;
        state.triedTranscodeFallback = true;
      }
    } catch (err) {
      console.debug("Fallback para metadados nativos do navegador:", err);
    }

    // Attach external local subtitle tracks (FR-014)
    populateSubtitles(video.subtitles || []);

    const savedProgress = getPlaybackProgress(video.id);
    let initialStartSec = 0;
    const knownDur = getEffectiveDuration();
    if (
      savedProgress &&
      !savedProgress.completed &&
      savedProgress.currentTime > 3 &&
      (knownDur === 0 || knownDur > savedProgress.currentTime + 3)
    ) {
      initialStartSec = Math.floor(savedProgress.currentTime);
    }

    if (state.isTranscodedStream && initialStartSec > 0) {
      state.transcodeBaseOffset = initialStartSec;
      v.src = `${video.streamUrl}?transcode=1&start=${initialStartSec}`;
      dom.resumeBannerText.textContent = `Retomado automaticamente de ${formatDuration(initialStartSec)}.`;
      dom.resumeBanner.classList.remove("hidden");
    } else if (state.isTranscodedStream) {
      v.src = `${video.streamUrl}?transcode=1`;
    } else {
      v.src = video.streamUrl;
    }

    v.volume = state.prefs.volume;
    v.muted = state.prefs.muted;
    v.playbackRate = state.prefs.playbackRate;
    dom.speedSelect.value = String(state.prefs.playbackRate);
    dom.autoplayNextCheckbox.checked = Boolean(state.prefs.autoPlayNext);
    updateSeekBarUI();

    const onLoadedMeta = () => {
      v.removeEventListener("loadedmetadata", onLoadedMeta);
      v.playbackRate = state.prefs.playbackRate;
      if (v.duration > 0 && Number.isFinite(v.duration) && state.totalDuration <= 0) {
        state.totalDuration = v.duration;
      }
      populateAudioTracks();

      if (
        !state.isTranscodedStream &&
        initialStartSec > 0 &&
        v.duration > initialStartSec + 3
      ) {
        v.currentTime = initialStartSec;
        dom.resumeBannerText.textContent = `Retomado automaticamente de ${formatDuration(initialStartSec)}.`;
        dom.resumeBanner.classList.remove("hidden");
      }

      updateSeekBarUI();
      const playPromise = v.play();
      if (playPromise && typeof playPromise.catch === "function") {
        playPromise.catch(() => {});
      }
    };

    v.addEventListener("loadedmetadata", onLoadedMeta);
    v.load();
  }

  function closePlayer() {
    cancelAutoplayCountdown();
    const v = dom.videoElement;
    const dur = getEffectiveDuration();
    const cur = getEffectiveCurrentTime();
    if (state.currentVideo && dur > 0 && !v.ended) {
      savePlaybackProgress(state.currentVideo.id, cur, dur, false);
    }
    state.currentVideo = null;
    state.currentVideoIndex = -1;
    state.transcodeBaseOffset = 0;
    state.totalDuration = 0;
    v.pause();
    v.removeAttribute("src");
    v.innerHTML = "";
    v.load();
    dom.playerModal.classList.add("hidden");
    // Refresh cards to show updated progress bars
    renderCatalog();
  }

  function populateSubtitles(subtitles) {
    const v = dom.videoElement;
    dom.subtitleSelect.innerHTML = '<option value="-1">Desativadas</option>';

    subtitles.forEach((sub, idx) => {
      const trackEl = document.createElement("track");
      trackEl.kind = "subtitles";
      trackEl.label = sub.label || `Legenda ${idx + 1}`;
      trackEl.srclang = sub.lang || "und";
      trackEl.src = sub.srcUrl;
      if (idx === 0) {
        trackEl.default = true;
      }
      v.appendChild(trackEl);

      const opt = document.createElement("option");
      opt.value = String(idx);
      opt.textContent = `${sub.label} (${sub.format.toUpperCase()})`;
      dom.subtitleSelect.appendChild(opt);
    });

    if (subtitles.length > 0) {
      dom.subtitleSelect.value = "0";
      setTimeout(() => applySubtitleSelection(0), 50);
    } else {
      dom.subtitleSelect.value = "-1";
    }
  }

  function applySubtitleSelection(selectedIndex) {
    const tracks = dom.videoElement.textTracks;
    if (!tracks) return;
    for (let i = 0; i < tracks.length; i++) {
      tracks[i].mode = i === selectedIndex ? "showing" : "disabled";
    }
  }

  function populateAudioTracks() {
    const v = dom.videoElement;
    const audioTracks = v.audioTracks;
    dom.audioTrackSelect.innerHTML = "";

    if (!audioTracks || audioTracks.length <= 1) {
      const opt = document.createElement("option");
      opt.value = "0";
      opt.textContent = "Padrão (1 faixa)";
      dom.audioTrackSelect.appendChild(opt);
      dom.audioTrackSelect.disabled = true;
      return;
    }

    dom.audioTrackSelect.disabled = false;
    for (let i = 0; i < audioTracks.length; i++) {
      const t = audioTracks[i];
      const opt = document.createElement("option");
      opt.value = String(i);
      const label = t.label || t.language || `Faixa ${i + 1}`;
      opt.textContent = label;
      if (t.enabled) {
        opt.selected = true;
      }
      dom.audioTrackSelect.appendChild(opt);
    }
  }

  function startAutoplayCountdown() {
    cancelAutoplayCountdown();
    const nextIdx = state.currentVideoIndex + 1;
    if (!state.prefs.autoPlayNext || nextIdx >= state.videos.length) {
      return;
    }

    const nextVideo = state.videos[nextIdx];
    state.autoplayRemaining = 5;
    dom.autoplayCountdown.textContent = String(state.autoplayRemaining);
    dom.autoplayNextTitle.textContent = nextVideo.title || nextVideo.name;
    dom.autoplayOverlay.classList.remove("hidden");

    state.autoplayTimer = setInterval(() => {
      state.autoplayRemaining -= 1;
      dom.autoplayCountdown.textContent = String(state.autoplayRemaining);
      if (state.autoplayRemaining <= 0) {
        cancelAutoplayCountdown();
        openPlayer(nextIdx);
      }
    }, 1000);
  }

  function cancelAutoplayCountdown() {
    if (state.autoplayTimer) {
      clearInterval(state.autoplayTimer);
      state.autoplayTimer = null;
    }
    dom.autoplayOverlay.classList.add("hidden");
  }

  // --- Visual Folder Picker Modal (FR-018, FR-019) ---

  async function openLibraryModal() {
    state.pickerSelectedPaths.clear();
    updatePickerSelectedCount();
    dom.pickerStatusMsg.classList.add("hidden");
    renderActiveRootsInModal();
    dom.libraryModal.classList.remove("hidden");

    if (state.browseBase?.enabled) {
      dom.pickerSection.classList.remove("hidden");
      await loadPickerDirectory("");
    } else {
      dom.pickerSection.classList.add("hidden");
    }
  }

  function closeLibraryModal() {
    dom.libraryModal.classList.add("hidden");
  }

  function renderActiveRootsInModal() {
    dom.activeRootsList.innerHTML = "";
    if (state.roots.length === 0) {
      const li = document.createElement("li");
      li.className = "active-root-item";
      li.textContent = "Nenhuma pasta ativa no momento. Selecione uma ou mais pastas abaixo.";
      dom.activeRootsList.appendChild(li);
      return;
    }

    state.roots.forEach((root) => {
      const li = document.createElement("li");
      li.className = "active-root-item";

      const infoDiv = document.createElement("div");
      infoDiv.className = "active-root-info";

      const nameStrong = document.createElement("strong");
      nameStrong.textContent = root.label;
      infoDiv.appendChild(nameStrong);

      const badge = document.createElement("span");
      badge.className = "source-badge";
      badge.textContent = root.source === "cli" ? "Inicialização (Fixo)" : "Salvo na Biblioteca";
      infoDiv.appendChild(badge);

      li.appendChild(infoDiv);

      if (root.removable) {
        const unlinkBtn = document.createElement("button");
        unlinkBtn.type = "button";
        unlinkBtn.className = "btn btn-danger btn-sm";
        unlinkBtn.textContent = "Desvincular";
        unlinkBtn.title = "Remove a pasta da biblioteca sem excluir nenhum arquivo do disco";
        unlinkBtn.addEventListener("click", () => unlinkRoot(root.id));
        li.appendChild(unlinkBtn);
      }

      dom.activeRootsList.appendChild(li);
    });
  }

  async function loadPickerDirectory(relPath) {
    try {
      const params = new URLSearchParams({ path: relPath || "" });
      const res = await apiGet("/api/v1/fs/dirs?" + params.toString());
      state.pickerCurrentPath = res.currentPath || "";
      renderPickerBreadcrumbs(res.breadcrumbs || []);
      renderPickerDirs(res.directories || []);
    } catch (err) {
      showPickerStatus(err.message || "Falha ao listar pastas do diretório-base.");
    }
  }

  function renderPickerBreadcrumbs(crumbs) {
    dom.pickerBreadcrumbsList.innerHTML = "";
    crumbs.forEach((item, idx) => {
      const li = document.createElement("li");
      li.className = "breadcrumb-item";
      if (idx === crumbs.length - 1) {
        const span = document.createElement("span");
        span.className = "breadcrumb-current";
        span.textContent = item.label;
        li.appendChild(span);
      } else {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "breadcrumb-btn";
        btn.textContent = item.label;
        btn.addEventListener("click", () => loadPickerDirectory(item.relPath || ""));
        li.appendChild(btn);
      }
      dom.pickerBreadcrumbsList.appendChild(li);
    });
  }

  function renderPickerDirs(dirs) {
    dom.pickerDirsList.innerHTML = "";
    if (dirs.length === 0) {
      const li = document.createElement("li");
      li.className = "picker-dir-item";
      li.textContent = "Nenhuma subpasta encontrada neste diretório.";
      dom.pickerDirsList.appendChild(li);
      return;
    }

    dirs.forEach((dir) => {
      const li = document.createElement("li");
      li.className = "picker-dir-item";

      const label = document.createElement("label");
      label.className = "picker-dir-check";

      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.disabled = Boolean(dir.alreadyAdded);
      cb.checked = Boolean(dir.alreadyAdded || state.pickerSelectedPaths.has(dir.relPath));

      cb.addEventListener("change", () => {
        if (cb.checked) {
          state.pickerSelectedPaths.add(dir.relPath);
        } else {
          state.pickerSelectedPaths.delete(dir.relPath);
        }
        updatePickerSelectedCount();
      });

      const nameSpan = document.createElement("span");
      nameSpan.textContent = dir.name + (dir.alreadyAdded ? " (Já na biblioteca)" : "");

      label.appendChild(cb);
      label.appendChild(nameSpan);
      li.appendChild(label);

      if (dir.hasSubdirs) {
        const openBtn = document.createElement("button");
        openBtn.type = "button";
        openBtn.className = "btn btn-secondary btn-sm";
        openBtn.textContent = "Abrir subpastas \u2192";
        openBtn.addEventListener("click", () => loadPickerDirectory(dir.relPath));
        li.appendChild(openBtn);
      }

      dom.pickerDirsList.appendChild(li);
    });
  }

  function updatePickerSelectedCount() {
    const count = state.pickerSelectedPaths.size;
    dom.pickerSelectedCount.textContent = `${count} pasta(s) marcada(s)`;
    dom.addSelectedDirsBtn.disabled = count === 0;
  }

  function showPickerStatus(msg) {
    if (!msg) {
      dom.pickerStatusMsg.classList.add("hidden");
      return;
    }
    dom.pickerStatusMsg.textContent = msg;
    dom.pickerStatusMsg.classList.remove("hidden");
  }

  async function submitSelectedPickerDirs(pathsArray) {
    if (!pathsArray || pathsArray.length === 0) return;
    try {
      const resp = await fetch("/api/v1/roots", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ paths: pathsArray }),
      });
      const data = await resp.json();
      if (!resp.ok) {
        throw new Error(data?.error?.message || "Erro ao adicionar pastas.");
      }

      state.roots = data.roots || [];
      state.pickerSelectedPaths.clear();
      updatePickerSelectedCount();
      renderActiveRootsInModal();
      await loadPickerDirectory(state.pickerCurrentPath);
      showPickerStatus("Pasta(s) adicionada(s) e salva(s) na biblioteca!");

      const newestRoot = state.roots.at(-1);
      if (newestRoot) {
        await loadRootsAndRefresh(newestRoot.id);
      }
    } catch (err) {
      showPickerStatus(err.message || "Falha ao adicionar pastas.");
    }
  }

  async function unlinkRoot(rootId) {
    try {
      const resp = await fetch("/api/v1/roots/" + encodeURIComponent(rootId), {
        method: "DELETE",
        headers: { Accept: "application/json" },
      });
      const data = await resp.json();
      if (!resp.ok) {
        throw new Error(data?.error?.message || "Erro ao desvincular pasta.");
      }
      state.roots = data.roots || [];
      renderActiveRootsInModal();
      await loadPickerDirectory(state.pickerCurrentPath);
      await loadRootsAndRefresh("");
    } catch (err) {
      showPickerStatus(err.message || "Falha ao desvincular pasta.");
    }
  }

  // --- Event Listeners ---

  function bindEvents() {
    dom.brandHome.addEventListener("click", () => {
      dom.searchInput.value = "";
      state.searchQuery = "";
      state.isSearching = false;
      dom.clearSearchBtn.classList.add("hidden");
      if (state.roots.length > 0) {
        browseFolder(state.roots[0].id, "");
      }
    });

    // Debounced Search (US3)
    let searchDebounce = null;
    dom.searchInput.addEventListener("input", () => {
      clearTimeout(searchDebounce);
      const val = dom.searchInput.value;
      searchDebounce = setTimeout(() => executeSearch(val), 220);
    });

    dom.clearSearchBtn.addEventListener("click", () => {
      dom.searchInput.value = "";
      executeSearch("");
    });

    dom.emptyResetSearchBtn.addEventListener("click", () => {
      dom.searchInput.value = "";
      executeSearch("");
    });

    // Sort Selector (US3)
    dom.sortSelect.value = state.prefs.sortBy;
    dom.sortSelect.addEventListener("change", () => {
      state.prefs.sortBy = dom.sortSelect.value;
      savePreferences();
      if (state.isSearching) {
        executeSearch(state.searchQuery);
      } else if (state.activeRootId) {
        browseFolder(state.activeRootId, state.currentPath);
      }
    });

    dom.alertRetryBtn.addEventListener("click", () => loadRootsAndRefresh(state.activeRootId));

    // Library Modal triggers (US1 / FR-018)
    dom.openLibraryModalBtn.addEventListener("click", openLibraryModal);
    dom.emptyAddFoldersBtn.addEventListener("click", openLibraryModal);
    dom.closeLibraryModalBtn.addEventListener("click", closeLibraryModal);

    dom.addSelectedDirsBtn.addEventListener("click", () => {
      submitSelectedPickerDirs(Array.from(state.pickerSelectedPaths));
    });

    dom.selectCurrentDirBtn.addEventListener("click", () => {
      submitSelectedPickerDirs([state.pickerCurrentPath || "."]);
    });

    // Video Player Events (US2 & US4)
    dom.closePlayerBtn.addEventListener("click", closePlayer);
    dom.errorBackToLibBtn.addEventListener("click", closePlayer);

    dom.prevVideoBtn.addEventListener("click", () => {
      if (state.currentVideoIndex > 0) {
        openPlayer(state.currentVideoIndex - 1);
      }
    });

    dom.nextVideoBtn.addEventListener("click", () => {
      if (state.currentVideoIndex + 1 < state.videos.length) {
        openPlayer(state.currentVideoIndex + 1);
      }
    });

    dom.restartFromBeginningBtn.addEventListener("click", () => {
      dom.resumeBanner.classList.add("hidden");
      seekToSeconds(0);
    });

    dom.playPauseBtn.addEventListener("click", () => {
      if (dom.videoElement.paused) {
        dom.videoElement.play().catch(() => {});
      } else {
        dom.videoElement.pause();
      }
    });

    dom.seekBackBtn.addEventListener("click", () => {
      seekToSeconds(getEffectiveCurrentTime() - 10);
    });

    dom.seekFwdBtn.addEventListener("click", () => {
      seekToSeconds(getEffectiveCurrentTime() + 10);
    });

    dom.seekSlider.addEventListener("input", () => {
      state.isDraggingSeek = true;
      dom.currentTimeLabel.textContent = formatDuration(Number(dom.seekSlider.value) || 0);
    });

    dom.seekSlider.addEventListener("change", () => {
      const targetSec = Number(dom.seekSlider.value) || 0;
      state.isDraggingSeek = false;
      seekToSeconds(targetSec);
    });

    dom.fullscreenBtn.addEventListener("click", () => {
      if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
      } else {
        dom.playerDialog.requestFullscreen().catch(() => {
          dom.videoElement.requestFullscreen().catch(() => {});
        });
      }
    });

    function showFullscreenControlsTemporarily() {
      if (!document.fullscreenElement) {
        clearTimeout(state.fsHideTimer);
        dom.playerDialog.classList.remove("fs-controls-visible", "fs-idle");
        return;
      }
      dom.playerDialog.classList.add("fs-controls-visible");
      dom.playerDialog.classList.remove("fs-idle");
      clearTimeout(state.fsHideTimer);

      const isHoveringControls =
        dom.playerBottomControls.matches(":hover") ||
        dom.playerDialog.querySelector(".player-header").matches(":hover");

      if (!dom.videoElement.paused && !isHoveringControls) {
        state.fsHideTimer = setTimeout(() => {
          if (document.fullscreenElement && !dom.videoElement.paused) {
            dom.playerDialog.classList.remove("fs-controls-visible");
            dom.playerDialog.classList.add("fs-idle");
          }
        }, 2500);
      }
    }

    dom.playerDialog.addEventListener("mousemove", showFullscreenControlsTemporarily);
    dom.playerDialog.addEventListener("touchstart", showFullscreenControlsTemporarily, { passive: true });
    dom.playerDialog.addEventListener("mouseleave", () => {
      if (document.fullscreenElement && !dom.videoElement.paused) {
        clearTimeout(state.fsHideTimer);
        dom.playerDialog.classList.remove("fs-controls-visible");
        dom.playerDialog.classList.add("fs-idle");
      }
    });
    document.addEventListener("fullscreenchange", () => {
      if (document.fullscreenElement) {
        dom.fullscreenBtn.innerHTML = "&#x26F6; Sair Tela Cheia";
        showFullscreenControlsTemporarily();
      } else {
        dom.fullscreenBtn.innerHTML = "&#x26F6; Tela Cheia";
        clearTimeout(state.fsHideTimer);
        dom.playerDialog.classList.remove("fs-controls-visible", "fs-idle");
      }
    });
    dom.videoElement.addEventListener("pause", showFullscreenControlsTemporarily);
    dom.videoElement.addEventListener("play", showFullscreenControlsTemporarily);

    dom.muteBtn.addEventListener("click", () => {
      dom.videoElement.muted = !dom.videoElement.muted;
      updateSeekBarUI();
    });

    dom.volumeSlider.addEventListener("input", () => {
      const vol = Number(dom.volumeSlider.value);
      dom.videoElement.volume = Math.min(1, Math.max(0, vol));
      dom.videoElement.muted = vol === 0;
      updateSeekBarUI();
    });

    dom.videoElement.addEventListener("click", () => {
      if (dom.videoElement.paused) {
        dom.videoElement.play().catch(() => {});
      } else {
        dom.videoElement.pause();
      }
    });

    dom.videoElement.addEventListener("dblclick", () => {
      dom.fullscreenBtn.click();
    });

    dom.speedSelect.addEventListener("change", () => {
      const rate = Number(dom.speedSelect.value) || 1.0;
      dom.videoElement.playbackRate = rate;
      state.prefs.playbackRate = rate;
      savePreferences();
    });

    dom.subtitleSelect.addEventListener("change", () => {
      applySubtitleSelection(Number(dom.subtitleSelect.value));
    });

    dom.audioTrackSelect.addEventListener("change", () => {
      const audioTracks = dom.videoElement.audioTracks;
      if (!audioTracks) return;
      const targetIdx = Number(dom.audioTrackSelect.value);
      for (let i = 0; i < audioTracks.length; i++) {
        audioTracks[i].enabled = i === targetIdx;
      }
    });

    dom.autoplayNextCheckbox.addEventListener("change", () => {
      state.prefs.autoPlayNext = dom.autoplayNextCheckbox.checked;
      savePreferences();
    });

    dom.autoplayPlayNowBtn.addEventListener("click", () => {
      const nextIdx = state.currentVideoIndex + 1;
      cancelAutoplayCountdown();
      if (nextIdx < state.videos.length) {
        openPlayer(nextIdx);
      }
    });

    dom.autoplayCancelBtn.addEventListener("click", cancelAutoplayCountdown);

    const v = dom.videoElement;
    v.addEventListener("play", updateSeekBarUI);
    v.addEventListener("timeupdate", () => {
      if (!state.currentVideo) return;
      updateSeekBarUI();
      const dur = getEffectiveDuration();
      if (dur <= 0) return;
      const cur = getEffectiveCurrentTime();
      const now = Date.now();
      if (now - state.lastProgressSaveAt >= 2000) {
        state.lastProgressSaveAt = now;
        savePlaybackProgress(state.currentVideo.id, cur, dur, false);
      }
    });

    v.addEventListener("pause", () => {
      updateSeekBarUI();
      const dur = getEffectiveDuration();
      const cur = getEffectiveCurrentTime();
      if (state.currentVideo && dur > 0 && !v.ended) {
        savePlaybackProgress(state.currentVideo.id, cur, dur, false);
      }
    });

    v.addEventListener("volumechange", () => {
      state.prefs.volume = v.volume;
      state.prefs.muted = v.muted;
      savePreferences();
      updateSeekBarUI();
    });

    v.addEventListener("ended", () => {
      const dur = getEffectiveDuration();
      if (state.currentVideo && dur > 0) {
        savePlaybackProgress(state.currentVideo.id, dur, dur, true);
      }
      startAutoplayCountdown();
    });

    v.addEventListener("error", () => {
      if (!state.currentVideo) return;
      if (!state.triedTranscodeFallback) {
        state.triedTranscodeFallback = true;
        state.isTranscodedStream = true;
        v.src = state.currentVideo.streamUrl + "?transcode=1";
        v.load();
        v.play().catch(() => {});
        return;
      }
      dom.playerErrorBanner.classList.remove("hidden");
    });

    window.addEventListener("pagehide", () => {
      const dur = getEffectiveDuration();
      const cur = getEffectiveCurrentTime();
      if (state.currentVideo && dur > 0 && !v.ended) {
        savePlaybackProgress(state.currentVideo.id, cur, dur, false);
      }
    });

    // Keyboard & Smart TV Remote shortcuts
    document.addEventListener("keydown", handleGlobalKeydown);
  }

  function handleGlobalKeydown(e) {
    if (!dom.playerModal.classList.contains("hidden")) {
      handlePlayerKeydown(e);
      return;
    }
    if (!dom.libraryModal.classList.contains("hidden") && e.key === "Escape") {
      e.preventDefault();
      closeLibraryModal();
    }
  }

  function handlePlayerKeydown(e) {
    if (e.key === "Escape") {
      e.preventDefault();
      closePlayer();
      return;
    }
    const tag = document.activeElement?.tagName || "";
    if (tag === "SELECT" || tag === "INPUT" || tag === "BUTTON") {
      return;
    }
    const v = dom.videoElement;
    if (e.key === " " || e.key === "k" || e.key === "K") {
      e.preventDefault();
      if (v.paused) v.play().catch(() => {});
      else v.pause();
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      seekToSeconds(getEffectiveCurrentTime() + 10);
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      seekToSeconds(getEffectiveCurrentTime() - 10);
    } else if (e.key === "f" || e.key === "F") {
      e.preventDefault();
      dom.fullscreenBtn.click();
    }
  }

  // Initialize
  bindEvents();
  loadRootsAndRefresh("");
})();
