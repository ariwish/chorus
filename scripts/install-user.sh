#!/usr/bin/env bash
set -euo pipefail

app_id=space.f1nn.chorus
app_name=Chorus
bin_dir="${HOME}/.local/bin"
data_dir="${XDG_DATA_HOME:-${HOME}/.local/share}"
schema_dir="${data_dir}/glib-2.0/schemas"
applications_dir="${data_dir}/applications"
icons_dir="${data_dir}/icons/hicolor"
binary="${bin_dir}/chorus"
schema="${schema_dir}/${app_id}.gschema.xml"
desktop="${applications_dir}/${app_id}.desktop"
icon="${icons_dir}/scalable/apps/${app_id}.svg"
symbolic_icon="${icons_dir}/symbolic/apps/${app_id}-symbolic.svg"

script_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

remove_installation() {
	rm -f -- "$binary" "$schema" "$desktop" "$icon" "$symbolic_icon"

	if command -v glib-compile-schemas >/dev/null 2>&1 && [[ -d "$schema_dir" ]]; then
		glib-compile-schemas "$schema_dir"
	fi
	if command -v update-desktop-database >/dev/null 2>&1 && [[ -d "$applications_dir" ]]; then
		update-desktop-database "$applications_dir"
	fi

	printf '%s\n' "Removed the user installation of ${app_name}."
}

if [[ "${1:-}" == "--uninstall" || "${1:-}" == "uninstall" ]]; then
	if [[ "$#" -ne 1 ]]; then
		printf 'Usage: %s [--uninstall]\n' "$0" >&2
		exit 2
	fi
	remove_installation
	exit 0
fi

if [[ "$#" -ne 0 ]]; then
	printf 'Usage: %s [--uninstall]\n' "$0" >&2
	exit 2
fi

for tool in go glib-compile-schemas install; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'Required command not found: %s\n' "$tool" >&2
		exit 1
	fi
done

mkdir -p -- "$bin_dir"
temp_binary="$(mktemp "${bin_dir}/.chorus.XXXXXX")"
trap 'rm -f -- "$temp_binary"' EXIT
(
	cd "$script_dir"
	go build -o "$temp_binary" ./cmd/chorus
)
install -Dm755 "$temp_binary" "$binary"
rm -f -- "$temp_binary"
trap - EXIT

install -Dm644 "${script_dir}/data/${app_id}.gschema.xml" "$schema"
glib-compile-schemas "$schema_dir"

install -Dm644 "${script_dir}/data/icons/hicolor/scalable/apps/${app_id}.svg" "$icon"
install -Dm644 "${script_dir}/data/icons/hicolor/symbolic/apps/${app_id}-symbolic.svg" "$symbolic_icon"

desktop_exec="${binary//\\/\\\\}"
desktop_exec="${desktop_exec//\"/\\\"}"
desktop_exec="${desktop_exec//%/%%}"
install -d "$applications_dir"
cat > "${desktop}.tmp" <<EOF
[Desktop Entry]
Type=Application
Name=${app_name}
Comment=Sing along to your music
Exec="${desktop_exec}"
Icon=${app_id}
Terminal=false
Categories=AudioVideo;Audio;Music;
Keywords=lyrics;sync;music;audio;song;mpris;karaoke;
StartupNotify=true
EOF
install -Dm644 "${desktop}.tmp" "$desktop"
rm -f -- "${desktop}.tmp"

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$applications_dir"
fi

printf '%s\n' \
	"Installed ${app_name} for this user." \
	"Find it by searching for Chorus in the app grid, or run: ${binary}" \
	"To uninstall, run: ${script_dir}/scripts/install-user.sh --uninstall"
