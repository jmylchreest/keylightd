# GitHub Actions workflows

## GNOME test base images

[Publish GNOME test images](gnome-test-images.yml) builds one image per supported
GNOME Shell major and publishes it to `ghcr.io/jmylchreest/keylightd-gnome-test`.
The images contain GNOME and the test tools. Each build starts with a
digest-pinned Arch image and synchronizes its packages with a dated Arch Linux
Archive snapshot, so Shell, Mutter, GJS and Mesa come from the same package set.
Publishing the result allows CI runners to share those installed dependencies
and avoid contacting the archive during a PR test.

The publisher runs when the build inputs change on `main` or the
`ci/gnome-test-images` maintenance branch. It can also be started from the
Actions tab. To add or refresh a version, update the snapshot and base digest
as needed in [the GNOME build script](../../contrib/gnome-extension/tests/test-gnome-shell.sh),
and update the publishing and test matrices when adding a major. Build locally
with the same entry point before publishing, for example:

```sh
GNOME_VERSION=51 bash contrib/gnome-extension/tests/test-gnome-shell.sh --build-base
```

Tags include the Shell major and snapshot date, for example
`51-20261028`. Record the published digest when adopting an image in CI.
Make the GHCR package public on first publication so PR jobs can pull it
without publishing credentials. Run the Wayland suite against each new image
before adopting it in the test workflow.
