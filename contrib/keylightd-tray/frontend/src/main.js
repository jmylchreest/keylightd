// Wails runtime bindings
let GetStatus, GetVersion, SetLightState, SetGroupState;

// Check if running in Wails or browser
if (window.go && window.go.main && window.go.main.App) {
  GetStatus = window.go.main.App.GetStatus;
  GetVersion = window.go.main.App.GetVersion;
  SetLightState = window.go.main.App.SetLightState;
  SetGroupState = window.go.main.App.SetGroupState;
} else {
  // Mock functions for browser testing
  console.warn("Wails runtime not available - using mock data");

  GetVersion = async () => "dev (browser)";

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
}

// Additional Wails bindings for group management
let CreateGroup,
  DeleteGroup,
  SetGroupLights,
  GetLights,
  SetInitialWindowHeight,
  SaveSettings,
  GetSettings,
  GetCustomCSS;

if (window.go && window.go.main && window.go.main.App) {
  CreateGroup = window.go.main.App.CreateGroup;
  DeleteGroup = window.go.main.App.DeleteGroup;
  SetGroupLights = window.go.main.App.SetGroupLights;
  GetLights = window.go.main.App.GetLights;
  SetInitialWindowHeight = window.go.main.App.SetInitialWindowHeight;
  SaveSettings = window.go.main.App.SaveSettings;
  GetSettings = window.go.main.App.GetSettings;
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
  GetLights = async () => [
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
  ];
  SetInitialWindowHeight = async (contentHeight) => {
    console.log(`SetInitialWindowHeight: ${contentHeight}`);
  };
  SaveSettings = async (settings) => {
    console.log(`SaveSettings:`, settings);
  };
  GetSettings = async () => ({
    connectionType: "socket",
    socketPath: "",
    apiUrl: "",
    apiKey: "",
  });
  GetCustomCSS = async () => "";
}

// State
let refreshInterval = null;
let debounceTimers = {};
let activeSliders = {}; // Track sliders being dragged
let refreshPaused = false; // Pause refresh during slider interaction
let settingsDirty = false; // Track if settings have been modified
let settingsUpdating = false; // Prevent concurrent settings panel updates
let isWindows = false; // Track if running on Windows

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
      `Version: ${version}`;
  } catch (e) {
    console.error("Failed to get version:", e);
  }

  // Setup settings panel
  setupSettings();

  // Setup master toggle
  setupMasterToggle();

  // Setup collapsible sections
  setupCollapsibleSections();

  // Setup custom CSS auto-reload
  setupCustomCssReload();

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

  // Start refresh interval
  refreshInterval = setInterval(refresh, 1000);
});

// Settings panel management
function setupSettings() {
  const settingsBtn = document.getElementById("settings-btn");
  const settingsCloseBtn = document.getElementById("settings-close-btn");
  const settingsPanel = document.getElementById("settings-panel");
  const testConnectionBtn = document.getElementById("test-connection-btn");
  const saveSettingsBtn = document.getElementById("save-settings-btn");

  settingsBtn.addEventListener("click", async () => {
    if (settingsUpdating) return;
    settingsUpdating = true;

    settingsPanel.classList.add("open");
    try {
      await updateVisibilityLists();
      await updateGroupEditor();
    } finally {
      settingsUpdating = false;
    }
  });

  // Setup group creation
  const createGroupBtn = document.getElementById("create-group-btn");
  const newGroupNameInput = document.getElementById("new-group-name");

  createGroupBtn.addEventListener("click", async () => {
    const name = newGroupNameInput.value.trim();
    if (name) {
      try {
        await CreateGroup(name);
        newGroupNameInput.value = "";
        await updateGroupEditor();
        await refresh();
      } catch (e) {
        console.error("Failed to create group:", e);
      }
    }
  });

  newGroupNameInput.addEventListener("keypress", (e) => {
    if (e.key === "Enter") {
      createGroupBtn.click();
    }
  });

  settingsCloseBtn.addEventListener("click", () => {
    settingsPanel.classList.remove("open");
  });

  testConnectionBtn.addEventListener("click", testConnection);
  saveSettingsBtn.addEventListener("click", saveSettings);

  // Load saved settings
  loadSettings();

  // Setup connection type toggle
  setupConnectionTypeToggle();
}

// Save settings to backend
async function saveSettings() {
  const saveBtn = document.getElementById("save-settings-btn");
  const originalText = saveBtn.textContent;
  saveBtn.textContent = "Saving...";
  saveBtn.disabled = true;

  const settings = {
    connectionType: document.getElementById("conn-socket").checked
      ? "socket"
      : "http",
    socketPath: document.getElementById("socket-path").value,
    apiUrl: document.getElementById("api-url").value,
    apiKey: document.getElementById("api-key").value,
  };

  try {
    await SaveSettings(settings);

    // Also save to localStorage for persistence
    localStorage.setItem("connectionType", settings.connectionType);
    localStorage.setItem("socketPath", settings.socketPath);
    localStorage.setItem("apiUrl", settings.apiUrl);
    localStorage.setItem("apiKey", settings.apiKey);

    saveBtn.textContent = "Saved";
    settingsDirty = false;

    // Test the new connection
    try {
      await refresh();
    } catch (refreshErr) {
      console.error("Refresh after save failed:", refreshErr);
      // Don't fail the save, connection might need time
    }

    // Reset button after delay
    setTimeout(() => {
      saveBtn.textContent = originalText;
      saveBtn.disabled = true;
    }, 1500);
  } catch (e) {
    console.error("Save settings error:", e);
    saveBtn.textContent = "Failed";

    // Re-enable button on error
    setTimeout(() => {
      saveBtn.textContent = originalText;
      saveBtn.disabled = false;
    }, 1500);
  }
}

// Load settings from localStorage
function loadSettings() {
  const socketPath = localStorage.getItem("socketPath") || "";
  const refreshMs = localStorage.getItem("refreshInterval") || "2500";
  const apiUrl = localStorage.getItem("apiUrl") || "";
  const apiKey = localStorage.getItem("apiKey") || "";

  document.getElementById("socket-path").value = socketPath;
  document.getElementById("refresh-interval").value = refreshMs;
  // Only set API URL if there's a saved value, otherwise keep the default
  if (apiUrl) {
    document.getElementById("api-url").value = apiUrl;
  }
  document.getElementById("api-key").value = apiKey;

  // Update refresh interval
  const interval = parseInt(refreshMs);
  if (interval >= 1000) {
    clearInterval(refreshInterval);
    refreshInterval = setInterval(refresh, interval);
  }

  // Setup change listeners - mark settings as dirty
  const markDirty = () => {
    settingsDirty = true;
    document.getElementById("save-settings-btn").disabled = false;
  };

  document
    .getElementById("refresh-interval")
    .addEventListener("change", (e) => {
      let interval = parseInt(e.target.value);
      if (interval < 1000) {
        interval = 1000;
        e.target.value = interval;
      }
      localStorage.setItem("refreshInterval", interval);
      clearInterval(refreshInterval);
      refreshInterval = setInterval(refresh, interval);
    });

  document.getElementById("socket-path").addEventListener("input", markDirty);
  document.getElementById("api-url").addEventListener("input", markDirty);
  document.getElementById("api-key").addEventListener("input", markDirty);
  document.getElementById("conn-socket").addEventListener("change", markDirty);
  document.getElementById("conn-http").addEventListener("change", markDirty);
}

// Test connection - tests with currently saved settings
async function testConnection() {
  const statusEl = document.getElementById("connection-status");
  statusEl.textContent = "Testing...";
  statusEl.className = "connection-status";

  try {
    await GetStatus();
    statusEl.textContent = "Connected";
    statusEl.className = "connection-status success";
  } catch (e) {
    console.error("Test connection error:", e);
    statusEl.textContent = "Failed";
    statusEl.className = "connection-status error";
  }
}

// Update visibility lists in settings
async function updateVisibilityLists() {
  try {
    const status = await GetStatus();

    // Lights visibility
    const lightsContainer = document.getElementById("visibility-lights");
    const visibleLights = JSON.parse(
      localStorage.getItem("visibleLights") || "[]",
    );

    let lightsHtml = "<h4>Lights</h4>";
    status.lights.forEach((light) => {
      const checked =
        visibleLights.length === 0 || visibleLights.includes(light.id);
      lightsHtml += `
                <div class="visibility-item">
                    <input type="checkbox" id="vis-light-${light.id}" ${checked ? "checked" : ""}
                        onchange="toggleVisibility('light', '${light.id}', this.checked)">
                    <label for="vis-light-${light.id}">${escapeHtml(light.name)}</label>
                </div>
            `;
    });
    lightsContainer.innerHTML = lightsHtml;

    // Groups visibility
    const groupsContainer = document.getElementById("visibility-groups");
    const visibleGroups = JSON.parse(
      localStorage.getItem("visibleGroups") || "[]",
    );

    let groupsHtml = "<h4>Groups</h4>";
    status.groups.forEach((group) => {
      const checked =
        visibleGroups.length === 0 || visibleGroups.includes(group.id);
      groupsHtml += `
                <div class="visibility-item">
                    <input type="checkbox" id="vis-group-${group.id}" ${checked ? "checked" : ""}
                        onchange="toggleVisibility('group', '${group.id}', this.checked)">
                    <label for="vis-group-${group.id}">${escapeHtml(group.name)}</label>
                </div>
            `;
    });
    groupsContainer.innerHTML = groupsHtml;
  } catch (e) {
    console.error("Failed to update visibility lists:", e);
  }
}

// Toggle visibility of a light or group
window.toggleVisibility = function (type, id, visible) {
  const key = type === "light" ? "visibleLights" : "visibleGroups";
  let items = JSON.parse(localStorage.getItem(key) || "[]");

  if (visible) {
    if (!items.includes(id)) {
      items.push(id);
    }
  } else {
    items = items.filter((i) => i !== id);
  }

  localStorage.setItem(key, JSON.stringify(items));
};

// Update group editor in settings
async function updateGroupEditor() {
  try {
    const status = await GetStatus();
    const lights = await GetLights();
    const container = document.getElementById("group-editor");

    if (status.groups.length === 0) {
      container.innerHTML =
        '<div class="empty-state">No groups configured</div>';
      return;
    }

    let html = "";
    for (const group of status.groups) {
      html += `
        <div class="group-item" data-group-id="${group.id}">
          <div class="group-item-header">
            <span class="group-item-name">${escapeHtml(group.name)}</span>
            <div class="group-item-actions">
              <button class="btn btn-danger" onclick="deleteGroup('${group.id}')">Delete</button>
            </div>
          </div>
          <div class="group-lights-list">
            ${lights
              .map((light) => {
                const checked =
                  group.lightIds && group.lightIds.includes(light.id);
                return `
                <div class="group-light-item">
                  <input type="checkbox" id="grp-${group.id}-${light.id}"
                    ${checked ? "checked" : ""}
                    onchange="toggleGroupLight('${group.id}', '${light.id}', this.checked)">
                  <label for="grp-${group.id}-${light.id}">${escapeHtml(light.name)}</label>
                </div>
              `;
              })
              .join("")}
          </div>
        </div>
      `;
    }
    container.innerHTML = html;
  } catch (e) {
    console.error("Failed to update group editor:", e);
  }
}

// Delete a group
window.deleteGroup = async function (groupId) {
  if (confirm("Are you sure you want to delete this group?")) {
    try {
      await DeleteGroup(groupId);
      await updateGroupEditor();
      await refresh();
    } catch (e) {
      console.error("Failed to delete group:", e);
    }
  }
};

// Toggle a light in a group
window.toggleGroupLight = async function (groupId, lightId, checked) {
  try {
    const status = await GetStatus();
    const group = status.groups.find((g) => g.id === groupId);
    if (!group) return;

    let lightIds = group.lightIds || [];
    if (checked) {
      if (!lightIds.includes(lightId)) {
        lightIds.push(lightId);
      }
    } else {
      lightIds = lightIds.filter((id) => id !== lightId);
    }

    await SetGroupLights(groupId, lightIds);
    await refresh();
  } catch (e) {
    console.error("Failed to update group lights:", e);
  }
};

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

// Setup connection type toggle
function setupConnectionTypeToggle() {
  const socketRadio = document.getElementById("conn-socket");
  const httpRadio = document.getElementById("conn-http");
  const socketSettings = document.querySelectorAll(".socket-setting");
  const httpSettings = document.querySelectorAll(".http-setting");
  const socketOption = socketRadio.closest(".setting-group");

  // Hide Unix socket option on Windows (not supported)
  if (isWindows && socketOption) {
    socketOption.style.display = "none";
    // Force HTTP mode on Windows
    httpRadio.checked = true;
    localStorage.setItem("connectionType", "http");
  }

  function updateConnectionFields() {
    const isSocket = socketRadio.checked;
    socketSettings.forEach(
      (el) => (el.style.display = isSocket ? "flex" : "none"),
    );
    httpSettings.forEach(
      (el) => (el.style.display = isSocket ? "none" : "flex"),
    );
    localStorage.setItem("connectionType", isSocket ? "socket" : "http");

    // Reset test connection result
    const statusEl = document.getElementById("connection-status");
    statusEl.textContent = "";
    statusEl.className = "connection-status";
  }

  socketRadio.addEventListener("change", updateConnectionFields);
  httpRadio.addEventListener("change", updateConnectionFields);

  // Load saved connection type (or default to HTTP on Windows)
  const savedType = localStorage.getItem("connectionType") || "socket";
  if (savedType === "http" || isWindows) {
    httpRadio.checked = true;
  } else {
    socketRadio.checked = true;
  }
  updateConnectionFields();

  // Setup API key show/hide toggle
  const apiKeyToggle = document.getElementById("api-key-toggle");
  const apiKeyInput = document.getElementById("api-key");

  if (apiKeyToggle && apiKeyInput) {
    apiKeyToggle.addEventListener("click", () => {
      const isPassword = apiKeyInput.type === "password";
      apiKeyInput.type = isPassword ? "text" : "password";

      // Toggle icon visibility
      const eyeIcon = apiKeyToggle.querySelector(".eye-icon");
      const eyeOffIcon = apiKeyToggle.querySelector(".eye-off-icon");
      if (eyeIcon && eyeOffIcon) {
        eyeIcon.style.display = isPassword ? "none" : "block";
        eyeOffIcon.style.display = isPassword ? "block" : "none";
      }
    });
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
  // Skip refresh if paused (during slider interaction)
  if (refreshPaused) {
    return;
  }

  try {
    const status = await GetStatus();
    updateUI(status);
    updateStatusBadge(status.onCount, status.total, true);
    updateMasterToggle(status.onCount);
    updateSectionVisibility(status);
  } catch (e) {
    console.error("Failed to refresh:", e);
    updateStatusBadge(0, 0, false);
    updateMasterToggle(0);
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
  const visibleGroups = JSON.parse(
    localStorage.getItem("visibleGroups") || "[]",
  );
  const filteredGroups =
    visibleGroups.length === 0
      ? status.groups
      : status.groups.filter((g) => visibleGroups.includes(g.id));

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
  const visibleGroups = JSON.parse(
    localStorage.getItem("visibleGroups") || "[]",
  );
  const filteredGroups =
    visibleGroups.length === 0
      ? groups
      : groups.filter((g) => visibleGroups.includes(g.id));

  if (filteredGroups.length === 0) {
    container.innerHTML = '<div class="empty-state">No groups visible</div>';
    return;
  }

  // Check if any slider is active - if so, update values without re-rendering
  const hasActiveSlider = Object.values(activeSliders).some((v) => v);
  if (hasActiveSlider) {
    updateSliderValues(filteredGroups, "group");
    return;
  }

  container.innerHTML = filteredGroups
    .map((group) => createControlCard(group, "group"))
    .join("");
}

// Render lights
function renderLights(lights) {
  const container = document.getElementById("lights-list");

  // Filter by visibility
  const visibleLights = JSON.parse(
    localStorage.getItem("visibleLights") || "[]",
  );
  const filteredLights =
    visibleLights.length === 0
      ? lights
      : lights.filter((l) => visibleLights.includes(l.id));

  if (filteredLights.length === 0) {
    container.innerHTML = '<div class="empty-state">No lights visible</div>';
    return;
  }

  // Check if any slider is active - if so, update values without re-rendering
  const hasActiveSlider = Object.values(activeSliders).some((v) => v);
  if (hasActiveSlider) {
    updateSliderValues(filteredLights, "light");
    return;
  }

  container.innerHTML = filteredLights
    .map((light) => createControlCard(light, "light"))
    .join("");
}

// Update slider values without full re-render
function updateSliderValues(items, type) {
  items.forEach((item) => {
    const card = document.querySelector(
      `.control-card[data-id="${item.id}"][data-type="${type}"]`,
    );
    if (!card) return;

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

// Create a control card HTML
function createControlCard(item, type) {
  const id = item.id;
  const name = item.name;
  const on = item.on;
  const brightness = item.brightness;
  const temperature = item.temperature;

  const details =
    type === "light"
      ? item.productName || "Key Light"
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
  refreshPaused = true;
};

// End slider drag - resume refresh after delay
window.endSliderDrag = function (key) {
  activeSliders[key] = false;

  // Resume refresh after a short delay to allow the change to be sent
  setTimeout(() => {
    // Only resume if no other sliders are active
    if (!Object.values(activeSliders).some((v) => v)) {
      refreshPaused = false;

      // Restart the refresh interval so next poll waits full duration
      const interval = parseInt(
        localStorage.getItem("refreshInterval") || "2500",
      );
      clearInterval(refreshInterval);
      refreshInterval = setInterval(refresh, interval);
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
