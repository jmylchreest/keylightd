export async function setupSecureConnection(refreshLights) {
  const app = window.go?.main?.App;
  if (!app?.HTTPSStatus) return;
  const get = (id) => document.getElementById(id);
  const dialog = get("secure-dialog");
  const confirm = get("secure-confirm");
  const cancel = get("secure-cancel");
  const code = get("secure-code");
  const address = get("secure-address");
  const socket = () => get("socket-path").value;
  let local;
  let busy = false;
  get("secure-panel").hidden = false;

  async function update() {
    const settings = await app.GetSettings();
    const paired = Boolean(settings.fingerprint);
    const viaSocket = settings.connectionType === "socket";
    const method = viaSocket ? "local" : paired ? "secure" : "manual";
    for (const id of ["local", "secure", "manual"]) {
      const row = get("method-" + id);
      row.querySelector(".pref-tail").textContent = id === method ? "✓ Active" : "›";
      row.classList.toggle("active-method", id === method);
    }
    get("summary-connection").textContent = get("method-" + method).querySelector("strong").textContent;
    get("socket-summary").textContent = settings.socketPath || "Default Unix socket";
    try {
      local = await app.HTTPSStatus(socket());
    } catch {
      local = null;
    }
    get("local-https").hidden = !local;
    if (!local) return;
    const selected = address.value;
    address.replaceChildren(...(local.urls || []).map((url, index) => {
      const host = new URL(url).hostname;
      const here = host === "[::1]" || host.startsWith("127.");
      const suffix = here ? " · this computer" : index === 0 ? " · suggested" : "";
      return new Option(url + suffix, url);
    }));
    if ([...address.options].some(option => option.value === selected)) address.value = selected;
    get("listener-status").textContent = local.enabled ? "HTTPS enabled" : local.restore_http ? "HTTPS off · HTTP enabled" : "HTTPS off · local socket only";
    get("enable-https").hidden = local.enabled && settings.fingerprint === local.fingerprint;
    get("enable-label").textContent = local.enabled ? "Use HTTPS" : "Enable and use HTTPS";
    get("issue-invitation").hidden = !local.enabled;
    get("disable-https").hidden = !local.enabled;
    get("forget-https").hidden = local.enabled || !local.fingerprint;
    get("chosen-address").textContent = address.value;
    get("identity-summary").textContent = local.fingerprint ? "Saved" : "No saved identity";
    get("fingerprint").textContent = local.fingerprint ? "SHA-256 · " + local.fingerprint : "No saved identity.";
  }

  function show(title, description, action, run, input) {
    get("secure-title").textContent = title;
    get("secure-description").textContent = description;
    get("secure-error").textContent = "";
    code.hidden = input === undefined;
    code.value = input || "";
    code.readOnly = Boolean(input);
    confirm.textContent = action;
    cancel.textContent = input ? "Done" : "Cancel";
    confirm.onclick = async () => {
      if (busy) return;
      busy = true;
      confirm.disabled = cancel.disabled = true;
      try {
        if (input) {
          try { await navigator.clipboard.writeText(code.value); confirm.textContent = "Copied"; }
          catch { code.select(); get("secure-description").textContent = "Copy the selected invitation."; }
        } else {
          get("secure-error").textContent = "";
          confirm.textContent = "Working…";
          const message = await run();
          dialog.close();
          get("secure-notice").textContent = message;
          await refreshLights();
        }
      } catch (error) {
        get("secure-error").textContent = String(error);
        confirm.textContent = action;
      } finally {
        busy = false;
        confirm.disabled = cancel.disabled = false;
        await update();
      }
    };
    dialog.showModal();
    if (input === "") code.focus();
  }
  address.onchange = () => { get("chosen-address").textContent = address.value; };
  cancel.onclick = () => dialog.close();
  dialog.addEventListener("cancel", event => { if (busy) event.preventDefault(); });
  dialog.addEventListener("close", () => { code.value = ""; confirm.onclick = null; });
  get("enable-https").onclick = () => show("Use HTTPS for this tray?",
    !local.enabled && local.restore_http
      ? "Existing HTTP clients will disconnect and need to reconnect using HTTPS."
      : "",
    "Enable and use", async () => { await app.EnableHTTPS(socket(), address.value); return "Connected over HTTPS."; });
  get("enter-invitation").onclick = () => show("Pair with keylightd",
    "Paste a single-use invitation from the daemon host.", "Pair and test",
    async () => { await app.PairClient(code.value.trim()); return "Connected over HTTPS."; }, "");
  get("issue-invitation").onclick = async () => {
    try {
      const invitation = await app.CreateInvitation(socket(), address.value);
      const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(invitation));
      const reference = Array.from(new Uint8Array(digest, 0, 4), byte => byte.toString(16).padStart(2, "0")).join("").toUpperCase();
      show("Pair another client", "Invitation " + reference + " · Single use · Expires in five minutes", "Copy invitation", null, invitation);
    } catch (error) { get("secure-notice").textContent = String(error); }
  };
  get("disable-https").onclick = () => show("Disable HTTPS?",
    (local.restore_http ? "Restore plain HTTP on the daemon’s API port." : "Return to local socket access only.") +
    " Paired connections will disconnect. The daemon will keep its identity.", "Disable HTTPS",
    async () => { await app.DisableHTTPS(socket()); return "HTTPS disabled."; });
  get("forget-https").onclick = () => show("Forget HTTPS identity?",
    "Clients will need to pair again after the next activation.", "Forget identity",
    async () => { await app.ForgetHTTPSIdentity(socket()); return "Saved identity removed."; });
  get("settings-btn").addEventListener("click", update);
  get("socket-path").addEventListener("change", update);
  window.addEventListener("connection-saved", update);
  // Migrate existing settings once. Paired credentials are stored by the native app.
  if (!(await app.HasSavedConnection()) && localStorage.getItem("connectionType") === "http" &&
      localStorage.getItem("apiUrl") && localStorage.getItem("apiKey")) {
    await app.SaveSettings({connectionType:"http",socketPath:socket(),apiUrl:localStorage.getItem("apiUrl"),apiKey:localStorage.getItem("apiKey")});
    localStorage.removeItem("apiKey");
  }
  if (!(await app.HasSavedConnection()) && localStorage.getItem("connectionType") === "socket" && socket()) {
    await app.SaveSettings({connectionType:"socket",socketPath:socket()});
  }
  const saved = await app.GetSettings();
  get("socket-path").value = saved.socketPath || socket();
  if (!saved.fingerprint && saved.connectionType === "http") {
    get("api-url").value = saved.apiUrl;
    get("api-key").value = saved.apiKey;
  }
  await update();
}
