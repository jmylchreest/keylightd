import Gio from "gi://Gio";
import GLib from "gi://GLib";
import Soup from "gi://Soup";

export const profilePath = GLib.build_filenamev([GLib.get_user_config_dir(), "keylightd", "gnome-pairing.json"]);

export async function runClient(args, input = "") {
  if (!GLib.find_program_in_path("keylightctl"))
    throw new Error("Install keylightctl to use a local socket or paired HTTPS connection.");
  const proc = Gio.Subprocess.new(["keylightctl", ...args], Gio.SubprocessFlags.STDIN_PIPE |
    Gio.SubprocessFlags.STDOUT_PIPE | Gio.SubprocessFlags.STDERR_PIPE);
  return new Promise((resolve, reject) => {
    proc.communicate_utf8_async(input, null, (child, result) => {
      try {
        const [, stdout, stderr] = child.communicate_utf8_finish(result);
        if (!child.get_successful()) {
          const message = stderr.split("\n").find(line => line.startsWith("Error:"));
          throw new Error(message || "keylightctl could not complete the request. Check the daemon and client versions.");
        }
        resolve(stdout.trim());
      } catch (error) { reject(error); }
    });
  });
}

export async function request(settings, path, method = "GET", body = null) {
  const args = settings.get_string("connection-method") === "paired"
    ? ["--profile", profilePath] : ["--socket", settings.get_string("socket-path")];
  return JSON.parse(await runClient([...args, "request", method, path], body ? JSON.stringify(body) : ""));
}

export async function testManual(session, address, key) {
  if (!/^https?:\/\//i.test(address) || !key) throw new Error("Enter an HTTP or HTTPS address and an API key.");
  const message = Soup.Message.new("GET", address + "/api/v1/lights");
  if (!message) throw new Error("Invalid API address.");
  message.add_flags(Soup.MessageFlags.NO_REDIRECT);
  message.get_request_headers().append("X-API-Key", key);
  await new Promise((resolve, reject) => session.send_and_read_async(message, GLib.PRIORITY_DEFAULT, null, (s, res) => {
    try {
      const bytes = s.send_and_read_finish(res);
      if (message.status_code !== 200) throw new Error("HTTP " + message.status_code);
      const lights = JSON.parse(new TextDecoder().decode(bytes.get_data()));
      if (!lights || typeof lights !== "object") throw new Error("Invalid light list.");
      resolve();
    } catch (error) { reject(error); }
  }));
}
