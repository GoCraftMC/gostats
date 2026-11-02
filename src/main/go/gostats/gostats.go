// Package gostats counts the blocks each player breaks. It shows the shape of a real Go mod: game events
// feed plain Go state, goroutines handle saving and a web server, and nothing but the event handlers and
// the command touches the game.
package gostats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoCraftMC/gocraft-sdk/command"
	"github.com/GoCraftMC/gocraft-sdk/event"
	"github.com/GoCraftMC/gocraft-sdk/gocraft"
	"github.com/GoCraftMC/gocraft-sdk/mc"
	"github.com/GoCraftMC/gocraft-sdk/registry"
	"github.com/GoCraftMC/gostats/src/main/go/gostats/tally"
)

// Config is stored in config/gostats.json.
type Config struct {
	// WebAddress is where the scoreboard is served. Empty turns it off. The default only accepts
	// connections from this machine; think before exposing it, as it has no authentication.
	WebAddress string `json:"webAddress"`
	// SaveSeconds is how often changed counts are written to disk.
	SaveSeconds int `json:"saveSeconds"`
}

var scores = tally.New()

func init() {
	gocraft.OnInitialize(func() {
		cfg := Config{WebAddress: "127.0.0.1:25580", SaveSeconds: 60}
		if err := gocraft.LoadConfig(&cfg); err != nil {
			gocraft.Log().Error("could not read config, using defaults", "err", err)
		}
		if cfg.SaveSeconds <= 0 {
			cfg.SaveSeconds = 60
		}

		savePath := filepath.Join(gocraft.GameDir(), "gostats", "scores.json")
		if err := scores.Load(savePath); err != nil {
			gocraft.Log().Error("could not read saved scores", "path", savePath, "err", err)
		}

		registerContent()
		registerCommand()

		event.BlockBroken(func(_ mc.Level, player mc.Player, _ mc.BlockPos, _ mc.BlockState) {
			scores.Add(player.Name())
		})

		var stopBackground context.CancelFunc
		event.ServerStarted(func(mc.Server) {
			ctx, cancel := context.WithCancel(context.Background())
			stopBackground = cancel
			saveEvery(ctx, savePath, time.Duration(cfg.SaveSeconds)*time.Second)
			serveScoreboard(ctx, cfg.WebAddress)
		})
		event.ServerStopping(func(mc.Server) {
			if stopBackground != nil {
				stopBackground()
			}
			if err := scores.Save(savePath); err != nil {
				gocraft.Log().Error("could not save scores", "err", err)
			}
		})
	})
}

func registerContent() {
	counter := registry.RegisterItem("tally_counter", registry.Item{
		MaxStack: 1,
		OnUse: func(ctx registry.UseItem) mc.InteractionResult {
			if !ctx.Level.IsClient() {
				ctx.Player.Send(fmt.Sprintf("You have broken %d blocks.", scores.Count(ctx.Player.Name())))
			}
			return mc.Success()
		},
	})
	registry.AddToTab(registry.TabTools, counter)
}

func registerCommand() {
	command.Register(
		command.Literal("gostats").
			Executes(func(c *command.Context) int {
				c.Reply(fmt.Sprintf("%s: %d blocks", c.SenderName(), scores.Count(c.SenderName())))
				return 1
			}).
			Then(command.Literal("top").Executes(func(c *command.Context) int {
				top := scores.Top(5)
				if len(top) == 0 {
					c.Reply("Nobody has broken a block yet.")
					return 1
				}

				rows := make([]string, len(top))
				for i, entry := range top {
					rows[i] = fmt.Sprintf("%d. %s: %d", i+1, entry.Player, entry.Blocks)
				}
				c.Reply(strings.Join(rows, "\n"))
				return len(top)
			})),
	)
}

func saveEvery(ctx context.Context, path string, interval time.Duration) {
	gocraft.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := scores.Save(path); err != nil {
					gocraft.Log().Error("could not save scores", "err", err)
				}
			}
		}
	})
}

// serveScoreboard reads only the tally, never the game, so it needs nothing from the server thread.
func serveScoreboard(ctx context.Context, address string) {
	if address == "" {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /scores", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(scores.Top(0)); err != nil {
			gocraft.Log().Warn("could not write scores", "err", err)
		}
	})

	// Listening here rather than in the goroutine reports a port clash right away. It is not fatal: the
	// rest of the mod works without the scoreboard.
	listener, err := net.Listen("tcp", address)
	if err != nil {
		gocraft.Log().Error("scoreboard is off", "address", address, "err", err)
		return
	}

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	gocraft.Go(func() {
		if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			gocraft.Log().Error("scoreboard stopped", "err", err)
		}
	})
	gocraft.Go(func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	})
	gocraft.Log().Info("scoreboard is up", "url", "http://"+listener.Addr().String()+"/scores")
}
