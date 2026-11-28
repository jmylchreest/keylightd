import Adw from "gi://Adw";
import Gtk from "gi://Gtk";

export function action(group, title, subtitle, icon, activate) {
  const row = new Adw.ActionRow({title, subtitle, activatable: true, use_markup: false});
  if (icon) row.add_prefix(new Gtk.Image({icon_name:icon}));
  row.add_suffix(new Gtk.Image({icon_name:"go-next-symbolic"}));
  row.connect("activated", () => activate(row));
  group.add(row);
  return row;
}

export function subpage(window, title) {
  const content = new Adw.PreferencesPage();
  const toolbar = new Adw.ToolbarView({content});
  toolbar.add_top_bar(new Adw.HeaderBar());
  window.push_subpage(new Adw.NavigationPage({title, child:toolbar}));
  const group = new Adw.PreferencesGroup();
  content.add(group);
  return {group, content};
}
