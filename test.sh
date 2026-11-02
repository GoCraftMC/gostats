#!/bin/sh
# Builds GoStats and runs its self-test in a real Minecraft server, in Fabric's headless test mode. Expects
# the gocraft repo next to this one, with its mod jar built.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
run="$here/build/test-run"
cache="$HOME/.gocraft/cache"
mc=26.3 loader=0.20.1 installer=1.1.2 api=0.165.0+26.3 gametest=4.0.35+3434d6d95d

cd "$here"
./gow build

rm -rf "$run"
mkdir -p "$run/mods/gomods" "$cache/servers" "$cache/mods"
cp ../gocraft/build/libs/gocraft-0.1.0.jar "$run/mods/"
cp build/libs/*.gomod "$run/mods/gomods/"

launcher="$cache/servers/fabric-server-$mc-$loader-$installer.jar"
apijar="$cache/mods/fabric-api-$api.jar"
gametestjar="$cache/mods/fabric-gametest-api-v1-$gametest.jar"
[ -f "$launcher" ] || curl -fsSL -o "$launcher" "https://meta.fabricmc.net/v2/versions/loader/$mc/$loader/$installer/server/jar"
[ -f "$apijar" ] || curl -fsSL -o "$apijar" "https://maven.fabricmc.net/net/fabricmc/fabric-api/fabric-api/${api%+*}%2B${api#*+}/fabric-api-$api.jar"
[ -f "$gametestjar" ] || curl -fsSL -o "$gametestjar" "https://maven.fabricmc.net/net/fabricmc/fabric-api/fabric-gametest-api-v1/${gametest%+*}%2B${gametest#*+}/fabric-gametest-api-v1-$gametest.jar"
cp "$apijar" "$gametestjar" "$run/mods/"

cd "$run"
fail() { echo "FAIL: $1 (see $run/$2)"; exit 1; }

GOSTATS_SELFTEST=play timeout "${TEST_TIMEOUT:-240}" java -Xmx2G -Dfabric-api.gametest=true -jar "$launcher" nogui > play.log 2>&1 || true
grep -E 'GoCraft ready|scoreboard is|GOSTATS-SELFTEST|System chat|panicked' play.log || true
grep -q 'GOSTATS-SELFTEST PASS play' play.log || fail "the self-test did not pass" play.log
grep -qE '[0-9]+\. .*: 7' play.log || fail "/gostats top did not reply" play.log
[ -f gostats/scores.json ] || fail "shutting down did not save scores" play.log

# A second launch of the same directory, with a fresh test world: the scores must come back from disk.
rm -rf world "Test Level"
GOSTATS_SELFTEST=restart timeout "${TEST_TIMEOUT:-240}" java -Xmx2G -Dfabric-api.gametest=true -jar "$launcher" nogui > restart.log 2>&1 || true
grep -E 'GOSTATS-SELFTEST|panicked' restart.log || true
grep -q 'GOSTATS-SELFTEST PASS restart' restart.log || fail "the scores did not survive a restart" restart.log

echo "GoStats passed"
