export function installLightSetup() {
  const dialog = document.querySelector("#light-setup-dialog");
  const form = dialog.querySelector("form");
  const title = dialog.querySelector("h2");
  const message = dialog.querySelector("p");
  const next = dialog.querySelector("#light-setup-next");
  const blink = dialog.querySelector("#light-setup-blink");
  const confirm = dialog.querySelector("#light-setup-confirm");
  const cancel = dialog.querySelector("#light-setup-cancel");
  const password = form.elements.password;
  const address = form.elements.peripheral;
  const ssid = form.elements.ssid;
  const security = form.elements.security;
  let target, confirmationId, active, phase, generation = 0;

  function showFields(nextPhase) {
    phase = nextPhase;
    const isWiFi = phase === "wifi";
    address.required = !isWiFi;
    address.closest("label").hidden = isWiFi;
    ssid.required = isWiFi;
    ssid.closest("label").hidden = !isWiFi;
    security.closest("label").hidden = !isWiFi;
    password.required = isWiFi && security.value !== "0";
    password.closest("label").hidden = !password.required;
    next.textContent = isWiFi ? "Join Wi-Fi" : "Next";
    form.hidden = next.hidden = false;
  }

  function request(action, options = {}) {
    const setup = window.go?.main?.App?.LightSetup;
    if (!setup) throw new Error("Connect the tray to keylightd to set up a light.");
    return setup(action, target, confirmationId || "", options);
  }

  function show(id, name) {
    target = id;
    confirmationId = "";
    active = false;
    cancel.textContent = "Cancel";
    form.reset();
    blink.hidden = confirm.hidden = true;
    title.textContent = name ? `Set up ${name}` : "Add light via Bluetooth (BLE)";
    message.textContent = "Put the light in pairing mode, then click Next to blink it.";
    showFields("address");
    dialog.showModal();
  }

  function updateControls(waiting) {
    blink.hidden = confirm.hidden = !waiting;
  }

  async function observe(run, status) {
    while (dialog.open && run === generation) {
      if (status.stage === "awaiting_confirmation") {
        confirmationId = status.confirmationId;
        updateControls(true);
        message.textContent = "Did the intended light blink?";
        return;
      }
      const stage = status.stage;
      if (stage === "awaiting_wifi") {
        confirmationId = status.wifiId;
        message.textContent = "Enter the Wi-Fi settings for the confirmed light.";
        showFields("wifi");
        return;
      }
      if (stage === "ready") {
        active = false;
        message.textContent = "Light added.";
        cancel.textContent = "Done";
        return;
      }
      if (stage !== "configuring" && stage !== "canceling") {
        active = false;
        const failures = {
          identity_mismatch: "The Bluetooth light did not match the selected light.",
          wifi_credentials_rejected: "The light rejected the Wi-Fi credentials.",
          wifi_join_failed: "The light could not join Wi-Fi.",
        };
        message.textContent = failures[status.failureReason] ||
          (stage === "timed_out" ? "Setup timed out." : stage === "canceled" ? "Setup canceled." : "Setup failed.");
        showFields("address");
        return;
      }
      message.textContent = "Configuring light…";
      await new Promise(resolve => setTimeout(resolve, 1000));
      if (run !== generation || !dialog.open) return;
      status = await request("status");
    }
  }

  function report(error) {
    message.textContent = error?.message || String(error);
  }

  form.elements.security.onchange = () => {
    password.required = form.elements.security.value !== "0";
    password.closest("label").hidden = !password.required;
    if (!password.required) password.value = "";
  };
  form.onsubmit = async event => {
    event.preventDefault();
    const run = ++generation;
    next.disabled = cancel.disabled = true;
    const joining = phase === "wifi";
    const options = joining
      ? { ssid: ssid.value, password: password.value, security: Number(security.value) }
      : { peripheral: address.value.trim() };
    password.value = "";
    try {
      if (joining) await request("wifi", options);
      const status = joining ? await request("status") : await request("start", options);
      active = true;
      cancel.disabled = false;
      form.hidden = next.hidden = true;
      await observe(run, status);
    } catch (error) {
      report(error);
    } finally {
      next.disabled = cancel.disabled = false;
    }
  };

  blink.onclick = async () => {
    blink.disabled = true;
    try { await request("blink"); }
    catch (error) { report(error); }
    finally { blink.disabled = false; }
  };
  confirm.onclick = async () => {
    updateControls(false);
    try {
      await request("confirm");
      await observe(generation, await request("status"));
    } catch (error) {
      updateControls(true);
      report(error);
    }
  };
  async function close() {
    try {
      if (active) await request("cancel");
      active = false;
      generation++;
      form.reset();
      dialog.close();
    } catch (error) {
      report(error);
    }
  }
  cancel.onclick = close;
  dialog.oncancel = event => {
    event.preventDefault();
    if (!cancel.disabled) close();
  };
  document.querySelector("#add-bluetooth-light").onclick = () => show("nearby");
  document.querySelector("#lights-list").addEventListener("click", event => {
    const card = event.target.closest(".light-setup-button")?.closest(".control-card");
    if (card) show(card.dataset.id, card.querySelector(".control-name").textContent);
  });
}
