# GoStats

A [GoCraft](https://github.com/GoCraftMC/gocraft) mod written in Go. It counts the blocks each player breaks and
shows them through a command, an item and a live web scoreboard.

- `/gostats` shows your count, and `/gostats top` shows the top five.
- Right-clicking with a Tally Counter (`/give @s gostats:tally_counter`) shows your count.
- `http://127.0.0.1:25580/scores` serves the scoreboard as JSON. Set `webAddress` in `config/gostats.json` to
  change the address, or leave it empty to turn it off. It has no authentication, so think before exposing it.
- Scores are saved to `gostats/scores.json` in the game directory.

```sh
./gow build      # build and run the unit tests
./gow runClient  # play with it
./test.sh        # run it in a real Minecraft server, twice, to test saving
```

`build.toml` and `test.sh` expect the `gocraft` repo next to this one, with its mod jar built.

## License

BSD-3-Clause.
