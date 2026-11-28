import { setupPreferences, visibleItems } from "./preferences.js";
import { setupSecureConnection } from "./secure.js";
// Wails runtime bindings
let GetStatus, GetVersion, GetDaemonVersion, SetLightState, SetGroupState;
let WatchAlive;

// Check if running in Wails or browser
if (window.go && window.go.main && window.go.main.App) {
  GetStatus = window.go.main.App.GetStatus;
  GetVersion = window.go.main.App.GetVersion;
  GetDaemonVersion = window.go.main.App.GetDaemonVersion;
  SetLightState = window.go.main.App.SetLightState;
  SetGroupState = window.go.main.App.SetGroupState;
  WatchAlive = window.go.main.App.WatchAlive;
} else {
  // Mock functions for browser testing
  console.warn("Wails runtime not available - using mock data");

  GetVersion = async () => "dev (browser)";
  GetDaemonVersion = async () => "dev (browser)";

  GetStatus = async () => ({
    lights: [
      {
        id: "light-1",
        name: "Key Light Left",
        on: true,
        brightness: 75,
        temperature: 4500,
        productName: "Key Light",
      },
      {
        id: "light-2",
        name: "Key Light Right",
        on: false,
        brightness: 50,
        temperature: 5000,
        productName: "Key Light",
      },
    ],
    groups: [
      {
        id: "group-1",
        name: "All Lights",
        lightIds: ["light-1", "light-2"],
        on: true,
        brightness: 75,
        temperature: 4500,
      },
    ],
    onCount: 1,
    offCount: 1,
    total: 2,
  });

  SetLightState = async (id, prop, value) => {
    console.log(`SetLightState: ${id} ${prop} = ${value}`);
  };

  SetGroupState = async (id, prop, value) => {
    console.log(`SetGroupState: ${id} ${prop} = ${value}`);
  };

  WatchAlive = async () => false;
}

// Additional Wails bindings for group management
let CreateGroup,
  DeleteGroup,
  SetGroupLights,
  SetInitialWindowHeight,
  GetCustomCSS;

if (window.go && window.go.main && window.go.main.App) {
  CreateGroup = window.go.main.App.CreateGroup;
  DeleteGroup = window.go.main.App.DeleteGroup;
  SetGroupLights = window.go.main.App.SetGroupLights;
  SetInitialWindowHeight = window.go.main.App.SetInitialWindowHeight;
  GetCustomCSS = window.go.main.App.GetCustomCSS;
} else {
  CreateGroup = async (name) => {
    console.log(`CreateGroup: ${name}`);
  };
  DeleteGroup = async (id) => {
    console.log(`DeleteGroup: ${id}`);
  };
  SetGroupLights = async (groupId, lightIds) => {
    console.log(`SetGroupLights: ${groupId} = ${lightIds}`);
  };
  SetInitialWindowHeight = async (contentHeight) => {
    console.log(`SetInitialWindowHeight: ${contentHeight}`);
  };
  GetCustomCSS = async () => "";
}

// State
let refreshInterval = null;
let debounceTimers = {};
let activeSliders = {}; // Track sliders being dragged
let isWindows = false; // Track if running on Windows

// Polling pause + diff infrastructure.
// pauseReasons collects every reason polling should be suspended
// (window hidden, slider being dragged, etc.) so multiple concurrent
// causes don't race each other. lastStatusHash skips DOM updates when
// state hasn't actually changed — most polls return identity, so the
// JS heap stays flat in steady state.
let pauseReasons = new Set();
let currentIntervalMs = 2500;
let lastStatusHash = null;

// When the daemon event stream is up, polling drops to a slow safety net
// and refreshes are driven by pushed events instead.
let userIntervalMs = 2500;
let watchAlive = false;
let watchRefreshTimer = null;
const WATCH_SAFETY_NET_MS = 90000;

// Tracks the *shape* of what's currently rendered (the sorted list of card
// IDs). When a refresh comes in with the same shape, we update card values
// in place via updateSliderValues — no innerHTML replacement, so we don't
// thrash the DOM or any focus/scroll state. Only when the shape actually
// changes (light added/removed, visibility toggled, etc.) do we fall back
// to the full re-render path.
const lastRenderedShape = { light: null, group: null };

function shapeHashFor(items) {
  return items
    .map((i) => i.id)
    .sort()
    .join(",");
}

function setPollingPaused(reason, paused) {
  if (paused) pauseReasons.add(reason);
  else pauseReasons.delete(reason);
  applyPauseState();
}

function applyPauseState() {
  if (pauseReasons.size > 0) {
    if (refreshInterval) {
      clearInterval(refreshInterval);
      refreshInterval = null;
    }
  } else if (!refreshInterval) {
    refresh(); // immediate refresh on resume so the user sees current state
    refreshInterval = setInterval(refresh, currentIntervalMs);
  }
}

function changePollingInterval(intervalMs) {
  userIntervalMs = intervalMs;
  applyEffectiveInterval();
}

function applyEffectiveInterval() {
  currentIntervalMs = watchAlive
    ? Math.max(WATCH_SAFETY_NET_MS, userIntervalMs)
    : userIntervalMs;
  if (refreshInterval) {
    clearInterval(refreshInterval);
    refreshInterval = setInterval(refresh, currentIntervalMs);
  }
}

function setupWatchEvents() {
  if (!window.runtime || !window.runtime.EventsOn) return;
  window.runtime.EventsOn("watch:status", (alive) => {
    watchAlive = !!alive;
    applyEffectiveInterval();
  });
  window.runtime.EventsOn("state:changed", () => {
    if (pauseReasons.size > 0) return; // hidden/dragging: refresh happens on resume
    if (watchRefreshTimer) clearTimeout(watchRefreshTimer);
    watchRefreshTimer = setTimeout(() => {
      watchRefreshTimer = null;
      refresh();
    }, 250);
  });
}

function setupVisibilityPause() {
  // When the Wails window is hidden via the tray, Go emits
  // "window:visible" so we can stop the polling loop. Without this,
  // the JS heap accumulates over hours of being hidden — webkit2gtk
  // doesn't aggressively reclaim under steady allocation pressure.
  if (!window.runtime || !window.runtime.EventsOn) return;
  window.runtime.EventsOn("window:visible", (visible) => {
    setPollingPaused("hidden", !visible);
  });
}

// Icons
const POWER_ICON = `<svg viewBox="0 0 24 24"><path d="M13 3h-2v10h2V3zm4.83 2.17l-1.42 1.42C17.99 7.86 19 9.81 19 12c0 3.87-3.13 7-7 7s-7-3.13-7-7c0-2.19 1.01-4.14 2.58-5.42L6.17 5.17C4.23 6.82 3 9.26 3 12c0 4.97 4.03 9 9 9s9-4.03 9-9c0-2.74-1.23-5.18-3.17-6.83z"/></svg>`;
const SUN_ICON = `<svg viewBox="0 0 24 24"><path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58c-.39-.39-1.03-.39-1.41 0-.39.39-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37c-.39-.39-1.03-.39-1.41 0-.39.39-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0 .39-.39.39-1.03 0-1.41l-1.06-1.06zm1.06-10.96c.39-.39.39-1.03 0-1.41-.39-.39-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36c.39-.39.39-1.03 0-1.41-.39-.39-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z"/></svg>`;
const TEMP_ICON = `<svg viewBox="0 0 24 24"><path d="M15 13V5c0-1.66-1.34-3-3-3S9 3.34 9 5v8c-1.21.91-2 2.37-2 4 0 2.76 2.24 5 5 5s5-2.24 5-5c0-1.63-.79-3.09-2-4zm-4-8c0-.55.45-1 1-1s1 .45 1 1h-1v1h1v2h-1v1h1v2h-2V5z"/></svg>`;

// Initialize
document.addEventListener("DOMContentLoaded", async () => {
  // Detect platform
  try {
    if (window.runtime && window.runtime.Environment) {
      const env = await window.runtime.Environment();
      isWindows = env.platform === "windows";
    }
  } catch (e) {
    console.error("Failed to get environment:", e);
  }

  try {
    const version = await GetVersion();
    document.getElementById("about-version").textContent =
      `Tray: ${version}`;
  } catch (e) {
    console.error("Failed to get tray version:", e);
  }

  try {
    const daemonVersion = await GetDaemonVersion();
    document.getElementById("about-daemon-version").textContent =
      `Daemon: ${daemonVersion || "unavailable"}`;
  } catch (e) {
    console.error("Failed to get daemon version:", e);
  }

  // Setup settings panel
  setupPreferences({ GetStatus, CreateGroup, DeleteGroup, SetGroupLights, changePollingInterval, isWindows,
    refresh: () => { lastStatusHash = null; return refresh(); },
  });
  await setupSecureConnection(refresh);

  // Setup master toggle
  setupMasterToggle();

  // Setup collapsible sections
  setupCollapsibleSections();

  // Setup custom CSS auto-reload
  setupCustomCssReload();

  // Hook window-visibility events from Go before first refresh so a
  // window that opens already-visible doesn't miss the initial event.
  setupVisibilityPause();
  setupWatchEvents();

  // Initial load
  await refresh();

  // Set initial window height based on content (only once on startup)
  const main = document.querySelector(".main");
  if (main) {
    try {
      await SetInitialWindowHeight(main.scrollHeight);
    } catch (e) {
      // Ignore errors in browser mode
    }
  }

  // Start the polling loop at the user's configured rate. Replaces the
  // old hardcoded 1000ms initial poll — never poll faster than the user
  // explicitly chose.
  userIntervalMs = parseInt(localStorage.getItem("refreshInterval") || "2500");
  if (userIntervalMs < 1000) userIntervalMs = 1000;
  // The watch may have connected before this listener registered.
  try {
    watchAlive = !!(await WatchAlive());
  } catch (e) {
    watchAlive = false;
  }
  applyEffectiveInterval();
  applyPauseState();
});

// Setup custom CSS auto-reload via filesystem watcher
function setupCustomCssReload() {
  // Load initial custom CSS
  loadCustomCss();

  // Listen for reload event from backend file watcher
  if (window.runtime && window.runtime.EventsOn) {
    window.runtime.EventsOn("reload-custom-css", () => {
      loadCustomCss();
    });
  }
}

// Load custom CSS from backend (XDG config directory)
async function loadCustomCss() {
  try {
    const cssText = await GetCustomCSS();

    // Get or create the custom style element
    let customStyle = document.getElementById("custom-css-style");
    if (!customStyle) {
      customStyle = document.createElement("style");
      customStyle.id = "custom-css-style";
      document.head.appendChild(customStyle);
    }

    if (cssText) {
      // Try to parse the CSS using CSSStyleSheet for validation
      const testSheet = new CSSStyleSheet();
      await testSheet.replace(cssText);

      // If we get here, CSS is valid - apply it
      customStyle.textContent = cssText;
      console.log("Custom CSS loaded successfully");
    } else {
      // No custom CSS file, clear any existing custom styles
      customStyle.textContent = "";
    }
  } catch (e) {
    console.error("Custom CSS load failed:", e.message);
  }
}

// Master toggle — toggle all lights on/off
function setupMasterToggle() {
  const btn = document.getElementById("master-toggle-btn");
  if (!btn) return;

  btn.addEventListener("click", async () => {
    try {
      const status = await GetStatus();
      // If any light is on, turn all off. Otherwise turn all on.
      const targetState = status.onCount === 0;
      const promises = status.lights.map((light) =>
        SetLightState(light.id, "on", targetState),
      );
      await Promise.all(promises);
      await refresh();
    } catch (e) {
      console.error("Failed to toggle all lights:", e);
    }
  });
}

// Update master toggle visual state
function updateMasterToggle(onCount) {
  const btn = document.getElementById("master-toggle-btn");
  if (!btn) return;
  if (onCount > 0) {
    btn.classList.add("all-on");
    btn.title = "Turn all lights off";
  } else {
    btn.classList.remove("all-on");
    btn.title = "Turn all lights on";
  }
}

// Collapsible sections
function setupCollapsibleSections() {
  document.querySelectorAll(".section").forEach((section) => {
    // Start expanded
    section.classList.add("expanded");

    const heading = section.querySelector("h2");
    if (heading) {
      heading.addEventListener("click", () => {
        section.classList.toggle("expanded");
      });
    }
  });
}

// Refresh data from backend
async function refresh() {
  if (pauseReasons.size > 0) return;

  try {
    const status = await GetStatus();
    // Skip the render path entirely when state hasn't changed since
    // the last poll. JSON.stringify is cheap for the small status
    // payload and avoids touching the DOM (and webkit's JS heap)
    // during steady state.
    const hash = JSON.stringify(status);
    if (hash === lastStatusHash) return;
    lastStatusHash = hash;

    updateUI(status);
    updateStatusBadge(status.onCount, status.total, true);
    updateMasterToggle(status.onCount);
    updateSectionVisibility(status);
  } catch (e) {
    console.error("Failed to refresh:", e);
    updateStatusBadge(0, 0, false);
    updateMasterToggle(0);
    lastStatusHash = null; // force re-render on next successful poll
  }
}

// Update status badge
function updateStatusBadge(on, total, connected) {
  const badge = document.getElementById("status-badge");
  if (!connected) {
    badge.textContent = "Disconnected";
    badge.className = "status-badge error";
  } else {
    badge.textContent = `${on}/${total} on`;
    badge.className = "status-badge connected";
  }
}

// Update the UI with new data
function updateUI(status) {
  renderGroups(status.groups);
  renderLights(status.lights);
}

// Hide sections with no visible items
function updateSectionVisibility(status) {
  const groupsSection = document.getElementById("groups-section");
  const filteredGroups = visibleItems("Groups", status.groups);

  if (groupsSection) {
    if (filteredGroups.length === 0) {
      groupsSection.classList.add("hidden");
    } else {
      groupsSection.classList.remove("hidden");
    }
  }
}

// Render groups
function renderGroups(groups) {
  const container = document.getElementById("groups-list");

  // Filter by visibility
  const filteredGroups = visibleItems("Groups", groups);

  if (filteredGroups.length === 0) {
    container.innerHTML = '<div class="empty-state">No groups visible</div>';
    lastRenderedShape.group = null;
    return;
  }

  const hasActiveSlider = Object.values(activeSliders).some((v) => v);
  if (hasActiveSlider) {
    updateSliderValues(filteredGroups, "group");
    return;
  }

  // Same set of groups as last render? Patch values in place. Avoids the
  // innerHTML reassignment / DOM-subtree replacement, which preserves focus
  // and scroll position and skips webkit's layout/paint of new nodes.
  const newShape = shapeHashFor(filteredGroups);
  if (newShape === lastRenderedShape.group) {
    updateSliderValues(filteredGroups, "group");
    return;
  }

  container.innerHTML = filteredGroups
    .map((group) => createControlCard(group, "group"))
    .join("");
  lastRenderedShape.group = newShape;
}

// Render lights
function renderLights(lights) {
  const container = document.getElementById("lights-list");

  // Filter by visibility
  const filteredLights = visibleItems("Lights", lights);

  if (filteredLights.length === 0) {
    container.innerHTML = '<div class="empty-state">No lights visible</div>';
    lastRenderedShape.light = null;
    return;
  }

  const hasActiveSlider = Object.values(activeSliders).some((v) => v);
  if (hasActiveSlider) {
    updateSliderValues(filteredLights, "light");
    return;
  }

  // Same set of lights as last render? Patch values in place — see the
  // comment in renderGroups for the rationale.
  const newShape = shapeHashFor(filteredLights);
  if (newShape === lastRenderedShape.light) {
    updateSliderValues(filteredLights, "light");
    return;
  }

  container.innerHTML = filteredLights
    .map((light) => createControlCard(light, "light"))
    .join("");
  lastRenderedShape.light = newShape;
}

// Update slider values without full re-render
function updateSliderValues(items, type) {
  items.forEach((item) => {
    const card = document.querySelector(
      `.control-card[data-id="${item.id}"][data-type="${type}"]`,
    );
    if (!card) return;
    if (type === "light") {
      const details = card.querySelector(".control-details");
      if (details) details.textContent = lightDetailsText(item);
    }

    // Update brightness slider if not active
    const brightnessKey = `brightness-${item.id}`;
    if (!activeSliders[brightnessKey]) {
      const brightnessSlider = card.querySelector(
        `input[data-slider-key="${brightnessKey}"]`,
      );
      if (brightnessSlider) {
        brightnessSlider.value = item.brightness;
        brightnessSlider.style.setProperty("--fill", `${item.brightness}%`);
        const label =
          brightnessSlider.parentElement.querySelector(".slider-value");
        if (label) label.textContent = `${item.brightness}%`;
      }
    }

    // Update temperature slider if not active
    const tempKey = `temperature-${item.id}`;
    if (!activeSliders[tempKey]) {
      const tempSlider = card.querySelector(
        `input[data-slider-key="${tempKey}"]`,
      );
      if (tempSlider) {
        tempSlider.value = item.temperature;
        const tempFill =
          ((item.temperature - 2900) / (7000 - 2900)) * 100;
        tempSlider.style.setProperty("--fill", `${tempFill}%`);
        const label = tempSlider.parentElement.querySelector(".slider-value");
        if (label) label.textContent = `${item.temperature}K`;
      }
    }

    // Update power button state
    const powerBtn = card.querySelector(".power-button");
    if (powerBtn) {
      powerBtn.className = `power-button ${item.on ? "on" : ""}`;
      powerBtn.onclick = () => togglePower(item.id, type, !item.on);
    }

    // Update card state
    card.className = `control-card ${item.on ? "on" : "off"}`;
  });
}

function lightDetailsText(item) {
  const model = item.productName || "Key Light";
  return item.experimental === true ? `${model} · Experimental` : model;
}

// Create a control card HTML
function createControlCard(item, type) {
  const id = item.id;
  const name = item.name;
  const on = item.on;
  const brightness = item.brightness;
  const temperature = item.temperature;

  const details =
    type === "light"
      ? lightDetailsText(item)
      : `${item.lightIds?.length || 0} lights`;

  const brightnessKey = `brightness-${id}`;
  const tempKey = `temperature-${id}`;

  const brightnessFill = brightness;
  const tempFill = ((temperature - 2900) / (7000 - 2900)) * 100;

  return `
        <div class="control-card ${on ? "on" : "off"}" data-id="${id}" data-type="${type}">
            <div class="control-header">
                <button class="power-button ${on ? "on" : ""}" onclick="togglePower('${id}', '${type}', ${!on})">
                    ${POWER_ICON}
                </button>
                <div class="control-info">
                    <div class="control-name">${escapeHtml(name)}</div>
                    <div class="control-details">${escapeHtml(details)}</div>
                </div>
            </div>
            <div class="slider-group">
                <div class="slider-row">
                    <span class="slider-icon">${SUN_ICON}</span>
                    <div class="slider-container">
                        <input type="range" class="slider brightness-slider" min="0" max="100" value="${brightness}"
                            style="--fill: ${brightnessFill}%"
                            data-slider-key="${brightnessKey}"
                            onmousedown="startSliderDrag('${brightnessKey}')"
                            onmouseup="endSliderDrag('${brightnessKey}')"
                            onmouseleave="endSliderDrag('${brightnessKey}')"
                            ontouchstart="startSliderDrag('${brightnessKey}')"
                            ontouchend="endSliderDrag('${brightnessKey}')"
                            onchange="setBrightness('${id}', '${type}', this.value)"
                            oninput="updateSliderFill(this)">
                        <span class="slider-value">${brightness}%</span>
                    </div>
                </div>
                <div class="slider-row">
                    <span class="slider-icon">${TEMP_ICON}</span>
                    <div class="slider-container">
                        <input type="range" class="slider temperature-slider" min="2900" max="7000" value="${temperature}"
                            style="--fill: ${tempFill}%"
                            data-slider-key="${tempKey}"
                            onmousedown="startSliderDrag('${tempKey}')"
                            onmouseup="endSliderDrag('${tempKey}')"
                            onmouseleave="endSliderDrag('${tempKey}')"
                            ontouchstart="startSliderDrag('${tempKey}')"
                            ontouchend="endSliderDrag('${tempKey}')"
                            onchange="setTemperature('${id}', '${type}', this.value)"
                            oninput="updateSliderFill(this)">
                        <span class="slider-value">${temperature}K</span>
                    </div>
                </div>
            </div>
        </div>
    `;
}

// Toggle power
window.togglePower = async function (id, type, on) {
  try {
    if (type === "group") {
      await SetGroupState(id, "on", on);
    } else {
      await SetLightState(id, "on", on);
    }
    // Immediate refresh for responsive feel
    await refresh();
  } catch (e) {
    console.error("Failed to toggle power:", e);
  }
};

// Set brightness with debounce
window.setBrightness = function (id, type, value) {
  const key = `brightness-${id}`;
  clearTimeout(debounceTimers[key]);

  debounceTimers[key] = setTimeout(async () => {
    try {
      const brightness = parseInt(value);
      if (type === "group") {
        await SetGroupState(id, "brightness", brightness);
      } else {
        await SetLightState(id, "brightness", brightness);
      }
    } catch (e) {
      console.error("Failed to set brightness:", e);
    }
  }, 150);
};

// Set temperature with debounce
window.setTemperature = function (id, type, value) {
  const key = `temperature-${id}`;
  clearTimeout(debounceTimers[key]);

  debounceTimers[key] = setTimeout(async () => {
    try {
      const temp = parseInt(value);
      if (type === "group") {
        await SetGroupState(id, "temperature", temp);
      } else {
        await SetLightState(id, "temperature", temp);
      }
    } catch (e) {
      console.error("Failed to set temperature:", e);
    }
  }, 150);
};

// Start slider drag - pause refresh
window.startSliderDrag = function (key) {
  activeSliders[key] = true;
  setPollingPaused("slider", true);
};

// End slider drag - resume refresh after a short delay to let the
// change reach the daemon, but only once no slider is still active.
window.endSliderDrag = function (key) {
  activeSliders[key] = false;
  setTimeout(() => {
    if (!Object.values(activeSliders).some((v) => v)) {
      setPollingPaused("slider", false);
    }
  }, 500);
};

// Update slider label and fill while dragging
window.updateSliderFill = function (slider) {
  const min = parseFloat(slider.min);
  const max = parseFloat(slider.max);
  const val = parseFloat(slider.value);
  const pct = ((val - min) / (max - min)) * 100;
  slider.style.setProperty("--fill", `${pct}%`);

  const valueSpan = slider.parentElement.querySelector(".slider-value");
  if (slider.classList.contains("brightness-slider")) {
    valueSpan.textContent = `${slider.value}%`;
  } else {
    valueSpan.textContent = `${slider.value}K`;
  }
};

// Escape HTML to prevent XSS
function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}
