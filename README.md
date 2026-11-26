# help

Ask a local [Ollama](https://ollama.com) model which shell command to run.

```sh
help what is my git remote url     # suggest a command, then run / edit / copy it
help                               # model manager: download, choose, delete models
help -p find big files here        # print only the command (also when piped)
help -m qwen2.5-coder:3b <question> # one-off model override
```

In the suggestion view: `enter` run · `e` edit · `c` copy · `r` retry · `q` quit.
Commands that delete or overwrite data are shown in red with a warning.

## Install

```sh
go install ./cmd/help   # installs to $(go env GOPATH)/bin/help
```

Needs Ollama running (`ollama serve` or the Ollama app). Config lives in
`~/.config/help-cli/config.json` (`model`, optional `host`; `$OLLAMA_HOST` is also respected).

Note: in **bash**, `help` is a shell builtin and wins over this binary; add
`alias help="$HOME/go/bin/help"` there. zsh has no such builtin.
