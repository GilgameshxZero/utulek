#!/bin/sh
# Build the Cygwin compatibility baseline into the local project tree.
set -eu

case "$(uname -s)" in
CYGWIN*) ;;
*)
	printf '%s\n' 'This baseline requires a Cygwin terminal on Windows.' >&2
	exit 1
	;;
esac

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

# Upstream make rules do not consistently quote paths.
case "$project_dir" in
*[[:space:]]*)
	printf '%s\n' 'Place the source in a directory without spaces for this baseline.' >&2
	exit 1
	;;
esac

for tool in gcc make dlltool pkg-config install
do
	command -v "$tool" >/dev/null 2>&1 || {
		printf 'Missing build tool: %s\n' "$tool" >&2
		exit 1
	}
done

jobs=${CMUS_WIN_JOBS:-2}
case "$jobs" in
''|*[!0-9]*|0)
	printf '%s\n' 'CMUS_WIN_JOBS must be a positive integer.' >&2
	exit 1
	;;
esac

# Always discard previous compiler/configuration objects before selecting this
# profile. This does not remove the installed tree or user music/configuration.
make distclean
./configure prefix="$project_dir/.obj/cygwin/install" bindir="$project_dir/.bin" DEBUG=0 \
	CONFIG_WAVEOUT=y \
	CONFIG_MAD=y CONFIG_FLAC=y CONFIG_VORBIS=y CONFIG_OPUS=y \
	CONFIG_WAV=y CONFIG_CUE=y \
	CONFIG_AAC=n CONFIG_AAUDIO=n CONFIG_ALSA=n CONFIG_AO=n \
	CONFIG_ARTS=n CONFIG_CDDB=n CONFIG_CDIO=n CONFIG_COREAUDIO=n \
	CONFIG_DISCID=n CONFIG_FFMPEG=n CONFIG_JACK=n CONFIG_MIKMOD=n \
	CONFIG_BASS=n CONFIG_MODPLUG=n CONFIG_MP4=n CONFIG_MPC=n \
	CONFIG_MPRIS=n CONFIG_OSS=n CONFIG_PULSE=n CONFIG_ROAR=n \
	CONFIG_SAMPLERATE=n CONFIG_SNDIO=n CONFIG_SUN=n CONFIG_TREMOR=n \
	CONFIG_VTX=n CONFIG_WAVPACK=n

# Do not derive the fork's version from the enclosing workspace repository.
make -j"$jobs" VERSION=cmus-win-0.1.0-dev
make install VERSION=cmus-win-0.1.0-dev
printf '\nBuilt Cygwin baseline: %s/.bin/cmus.exe\n' "$project_dir"
printf '%s\n' 'Requires the installed Cygwin runtime; this is not a standalone native build.'
