const get = id => document.getElementById(id);

// Older installations stored an empty list to mean every device was visible.
export function visibleItems(kind, items) {
  const ids = JSON.parse(localStorage.getItem("visible" + kind) || "[]");
  const explicit = localStorage.getItem("visibilityMode" + kind) === "selected";
  return !explicit && !ids.length ? items : items.filter(item => ids.includes(item.id));
}

function setVisible(kind, ids) {
  localStorage.setItem("visible" + kind, JSON.stringify(ids));
  localStorage.setItem("visibilityMode" + kind, "selected");
}

function prefRow(title, detail, control) {
  const el = document.createElement(control ? "label" : "button");
  el.className = "pref-row";
  const copy = document.createElement("span");
  copy.className = "pref-copy";
  const name = document.createElement("strong");
  name.textContent = title;
  const sub = document.createElement("small");
  sub.textContent = detail;
  copy.append(name, sub);
  const tail = control || document.createElement("span");
  if (!control) tail.textContent = "›";
  el.append(copy, tail);
  return el;
}

function toggle(on) {
  const input = document.createElement("input");
  input.type = "checkbox";
  input.setAttribute("role", "switch");
  input.checked = on;
  return input;
}

export function setupPreferences({GetStatus, CreateGroup, DeleteGroup, SetGroupLights, changePollingInterval, isWindows, refresh}) {
  const app = window.go?.main?.App;
  const panel = get("settings-panel");
  const notice = get("secure-notice");
  const stack = [];
  let status = {lights: [], groups: []};
  let group;
  let members;
  let groupVisible;

  function show(page, push = true) {
    if (push) stack.push(page);
    notice.textContent = "";
    panel.querySelectorAll("[data-view]").forEach(el => { el.hidden = el.dataset.view !== page; });
    get("prefs-back").style.visibility = stack.length > 1 ? "visible" : "hidden";
    panel.querySelector(".settings-content").scrollTop = 0;
    const heading = panel.querySelector("[data-view]:not([hidden]) h2");
    heading.tabIndex = -1;
    heading.focus({preventScroll:true});
  }

  function back() {
    if (stack.length < 2) return close();
    stack.pop();
    show(stack.at(-1), false);
  }

  function close() {
    panel.inert = true;
    panel.classList.remove("open");
    get("settings-btn").focus();
  }

  panel.addEventListener("click", event => {
    const button = event.target.closest("[data-page]");
    if (button) show(button.dataset.page);
  });
  panel.addEventListener("keydown", event => {
    if (event.key === "Escape") back();
  });
  get("prefs-back").onclick = back;
  get("settings-close-btn").onclick = close;
  get("settings-btn").addEventListener("click", async () => {
    panel.inert = false;
    panel.classList.add("open");
    stack.length = 0;
    show("index");
    try { await load(); } catch (error) { notice.textContent = String(error); }
  });

  function render() {
    const visible = visibleItems("Lights", status.lights).map(light => light.id);
    get("summary-lights").textContent = visible.length + " visible";
    get("summary-groups").textContent = status.groups.length + (status.groups.length === 1 ? " group" : " groups");
    get("visibility-lights").replaceChildren(...status.lights.map(light => {
      const input = toggle(visible.includes(light.id));
      input.onchange = () => {
        const ids = new Set(visible);
        if (input.checked) ids.add(light.id); else ids.delete(light.id);
        updateLights([...ids]);
      };
      return prefRow(light.name, [light.productName, light.ip].filter(Boolean).join(" · "), input);
    }));
    get("group-editor").replaceChildren(...status.groups.map(item => {
      const shown = visibleItems("Groups", status.groups).includes(item);
      const n = item.lightIds?.length || 0;
      const row = prefRow(item.name, n + (n === 1 ? " light" : " lights") + (shown ? " · Visible" : " · Hidden"));
      row.onclick = () => {
        group = item;
        members = new Set(group.lightIds);
        groupVisible = toggle(shown);
        get("group-members").replaceChildren(...status.lights.map(light => {
          const input = toggle(members.has(light.id));
          input.onchange = () => input.checked ? members.add(light.id) : members.delete(light.id);
          return prefRow(light.name, light.productName, input);
        }), prefRow("Show in the tray", "On this computer", groupVisible));
        panel.querySelector('[data-view="edit"] h2').textContent = group.name;
        show("edit");
      };
      return row;
    }));
  }
  async function load() {
    status = await GetStatus();
    render();
  }
  function updateLights(ids) {
    setVisible("Lights", ids);
    render();
    refresh();
  }
  get("show-lights").onclick = () => {
    localStorage.removeItem("visibleLights");
    localStorage.removeItem("visibilityModeLights");
    render();
    refresh();
  };
  get("hide-lights").onclick = () => updateLights([]);

  function action(id, fn) {
    const button = get(id);
    button.onclick = async () => {
      button.disabled = true;
      notice.textContent = "";
      try { await fn(); } catch (error) { notice.textContent = String(error); }
      finally { button.disabled = false; }
    };
  }
  action("create-group", async () => {
    const input = get("new-group-name");
    if (!input.value.trim()) return input.focus();
    await CreateGroup(input.value.trim());
    input.value = "";
    back();
    await load();
    await refresh();
  });
  action("save-group", async () => {
    await SetGroupLights(group.id, [...members]);
    const ids = new Set(visibleItems("Groups", status.groups).map(item => item.id));
    if (groupVisible.checked !== ids.has(group.id)) {
      if (groupVisible.checked) ids.add(group.id); else ids.delete(group.id);
      setVisible("Groups", [...ids]);
    }
    back();
    await load();
    await refresh();
  });
  action("delete-group", async () => {
    if (!window.confirm("Delete " + group.name + " for all clients?")) return;
    await DeleteGroup(group.id);
    back();
    await load();
    await refresh();
  });

  get("method-local").hidden = isWindows;
  get("socket-path").value = localStorage.getItem("socketPath") || "";
  get("api-url").value = localStorage.getItem("apiUrl") || "http://localhost:9123";
  for (const method of ["local", "manual"]) {
    action("use-" + method, async () => {
      if (!app) throw new Error("A running tray is required to test a connection.");
      const socketPath = get("socket-path").value.trim();
      await app.TestAndUseSettings({connectionType: method === "local" ? "socket" : "http", socketPath,
        apiUrl: get("api-url").value.trim(), apiKey: get("api-key").value});
      window.dispatchEvent(new Event("connection-saved"));
      back();
      notice.textContent = "Connected.";
      await load();
      await refresh();
    });
  }

  const interval = get("refresh-interval");
  interval.value = Number(localStorage.getItem("refreshInterval") || 2500) / 1000;
  interval.onchange = () => {
    if (!interval.reportValidity()) return;
    const ms = Math.round(interval.valueAsNumber * 1000);
    localStorage.setItem("refreshInterval", ms);
    changePollingInterval(ms);
  };
}
