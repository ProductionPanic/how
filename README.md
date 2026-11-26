# how

Ask a local [Ollama](https://ollama.com) model which shell command to run.

```sh
how do I see my git remote url     # suggest a command, then run / edit / copy it
how                                # model manager: download, choose, delete models
how -p find big files here         # print only the command (also when piped)
how -m qwen2.5-coder:3b <question> # one-off model override
```

In the suggestion view: `enter` run · `e` edit · `c` copy · `r` retry · `q` quit.
Commands that delete or overwrite data are shown in red with a warning.

## Install

```sh
brew install productionpanic/tap/how
```

Or with Go:

```sh
go install github.com/ProductionPanic/how/cmd/how@latest
```

Needs [Ollama](https://ollama.com/download) (`brew install ollama`). If it is
installed but not running, `how` offers to start it (the Ollama app on macOS,
otherwise `ollama serve` in the background, logging to `~/Library/Caches/how/ollama-serve.log`
on macOS). In the model manager, press `s` to start it.

Config lives in
`~/.config/how/config.json` (`model`, optional `host`; `$OLLAMA_HOST` is also respected).

Tip: zsh treats `?` as a glob, so quote questions that end in one: `how "what's my ip?"`.

## Releasing

Push a version tag; the Release workflow builds binaries with GoReleaser,
publishes a GitHub release and updates the cask in
[ProductionPanic/homebrew-tap](https://github.com/ProductionPanic/homebrew-tap):

```sh
git tag v0.2.0 && git push origin v0.2.0
```
