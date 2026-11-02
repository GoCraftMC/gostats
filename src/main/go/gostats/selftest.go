package gostats

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GoCraftMC/gocraft-sdk/event"
	"github.com/GoCraftMC/gocraft-sdk/gocraft"
	"github.com/GoCraftMC/gocraft-sdk/jvm"
	"github.com/GoCraftMC/gocraft-sdk/mc"
	"github.com/GoCraftMC/gostats/src/main/go/gostats/tally"
)

// selfTestBreaks is how many blocks the first self-test run breaks.
const selfTestBreaks = 7

// With GOSTATS_SELFTEST set the mod tests itself on server start, then stops the server. See test.sh.
// "play" uses a Fabric FakePlayer, which goes through the game's real block breaking code. "restart" runs
// on the next launch and checks that what "play" did was saved when the server shut down.
func init() {
	mode := os.Getenv("GOSTATS_SELFTEST")
	if mode == "" {
		return
	}
	gocraft.OnInitialize(func() {
		// Registered after the handler in gostats.go, so the scoreboard is already up.
		event.ServerStarted(func(server mc.Server) {
			test := selfTest
			if mode == "restart" {
				test = restartTest
			}
			if err := test(server); err != nil {
				gocraft.Log().Error("GOSTATS-SELFTEST FAIL: " + err.Error())
			} else {
				gocraft.Log().Info("GOSTATS-SELFTEST PASS " + mode)
			}
			server.RunCommand("stop")
		})
	})
}

func fetchScoreboard() ([]tally.Entry, error) {
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get("http://127.0.0.1:25580/scores")
	if err != nil {
		return nil, fmt.Errorf("scoreboard request: %w", err)
	}
	defer response.Body.Close()

	var rows []tally.Entry
	if err := json.NewDecoder(response.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("scoreboard JSON: %w", err)
	}
	return rows, nil
}

func restartTest(mc.Server) error {
	rows, err := fetchScoreboard()
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0].Blocks != selfTestBreaks {
		return fmt.Errorf("after a restart the scoreboard says %v", rows)
	}
	return nil
}

func selfTest(server mc.Server) error {
	const gameModeClass = "net.minecraft.server.level.ServerPlayerGameMode"

	level := server.Overworld()
	player := jvm.FindClass("net.fabricmc.fabric.api.entity.FakePlayer").
		StaticMethod("get", "(Lnet/minecraft/server/level/ServerLevel;)Lnet/fabricmc/fabric/api/entity/FakePlayer;").CallStatic(level).Object()
	name := mc.Player{Object: player}.Name()
	gameMode := jvm.FindClass("net.minecraft.server.level.ServerPlayer").Field("gameMode", "Lnet/minecraft/server/level/ServerPlayerGameMode;").Get(player)
	destroy := jvm.FindClass(gameModeClass).Method("destroyBlock", "(Lnet/minecraft/core/BlockPos;)Z")
	stone := jvm.FindClass("net.minecraft.world.level.block.Blocks").StaticField("STONE", "Lnet/minecraft/world/level/block/Block;").GetStatic()
	state := mc.Block{Object: stone.Object()}.DefaultState()

	first := mc.Pos(0, 200, 0)
	jvm.FindClass("net.minecraft.world.level.LevelReader").Method("getChunk", "(Lnet/minecraft/core/BlockPos;)Lnet/minecraft/world/level/chunk/ChunkAccess;").Call(level, first)

	for x := range selfTestBreaks {
		pos := mc.Pos(x, 200, 0)
		if !level.SetBlock(pos, state) {
			return fmt.Errorf("could not place stone at x=%d", x)
		}
		if !destroy.Call(gameMode, pos).Bool() {
			return fmt.Errorf("could not break stone at x=%d", x)
		}
	}
	if got := scores.Count(name); got != selfTestBreaks {
		return fmt.Errorf("the tally has %d breaks for %q, want %d", got, name, selfTestBreaks)
	}

	rows, err := fetchScoreboard()
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0].Player != name || rows[0].Blocks != selfTestBreaks {
		return fmt.Errorf("the scoreboard says %v", rows)
	}

	// test.sh checks the replies to these in the log.
	server.RunCommand("gostats")
	server.RunCommand("gostats top")

	counter := jvm.FindClass("net.minecraft.core.registries.BuiltInRegistries").StaticField("ITEM", "Lnet/minecraft/core/DefaultedRegistry;").GetStatic()
	item := jvm.FindClass("net.minecraft.core.Registry").Method("getValue", "(Lnet/minecraft/resources/Identifier;)Ljava/lang/Object;").Call(counter, mc.ID("tally_counter")).Object()
	if !strings.Contains(item.ClassOf().String(), "com.go.craft.generated") {
		return fmt.Errorf("tally_counter resolved to %s", item.ClassOf())
	}

	gocraft.Log().Info(fmt.Sprintf("GOSTATS-SELFTEST player=%s breaks=%d", name, selfTestBreaks))
	return nil
}
