# completion


Generate shell completion scripts for overseer.

## Installed with Homebrew

Nothing to run. The Homebrew cask generates completions for bash, zsh and fish when it installs overseer, and upgrades them with it. Your shell only needs to load Homebrew's completions folder.

### Zsh

`eval "$(brew shellenv)"` adds Homebrew's `share/zsh/site-functions` folder to `fpath`, where `_overseer` lives. It has to run **before** `compinit`. With oh-my-zsh, that means before `source $ZSH/oh-my-zsh.sh`, which runs `compinit` for you:

```bash [~/.zshrc]
eval "$(/opt/homebrew/bin/brew shellenv)"      # macOS; on Linux/WSL: /home/linuxbrew/.linuxbrew/bin/brew
source $ZSH/oh-my-zsh.sh                       # or: autoload -U compinit && compinit
```

If `overseer <Tab>` doesn't complete:

```bash
ls "$(brew --prefix)/share/zsh/site-functions/_overseer"   # installed?
print -l $fpath | grep site-functions                        # on fpath?
rm -f ~/.zcompdump* && exec zsh                              # clear zsh's completion cache
```

### Bash and fish

Bash needs the `bash-completion@2` package and a few lines in `~/.bash_profile` to load Homebrew's completions. With fish installed through Homebrew, completions work as is; another fish needs Homebrew's `vendor_completions.d` folder added. Homebrew's [shell completion guide](https://docs.brew.sh/Shell-Completion) has the exact lines for both.

## Other installs

For a binary installed with `overseer update`, a release download or `just dev`, `overseer completion <shell>` prints the script for you to install.

### Zsh

Completion must be enabled (`compinit`). oh-my-zsh does this; otherwise add `autoload -U compinit && compinit` to `~/.zshrc`.

Save the script into a folder on `fpath`, then start a new shell:

```bash
overseer completion zsh > "${fpath[1]}/_overseer"
```

Or load it on every start, by adding this line to `~/.zshrc` **after** `compinit` (or after `source $ZSH/oh-my-zsh.sh`):

```bash [~/.zshrc]
source <(overseer completion zsh)
```

### Bash

Needs the `bash-completion` package (`apt install bash-completion`, or `brew install bash-completion@2` on macOS).

```bash
echo 'source <(overseer completion bash)' >> ~/.bashrc
```

### Fish

```bash
overseer completion fish > ~/.config/fish/completions/overseer.fish
```

### PowerShell

```powershell
overseer completion powershell | Out-String | Invoke-Expression
```

Add the same line to your PowerShell profile (`$PROFILE`) to load it in every session.
