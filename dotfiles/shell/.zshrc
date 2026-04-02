# If you come from bash you might have to change your $PATH.
# export PATH=$HOME/bin:$HOME/.local/bin:/usr/local/bin:$PATH

# Path to your Oh My Zsh installation.
export ZSH="$HOME/.oh-my-zsh"

# Set name of the theme to load --- if set to "random", it will
# load a random theme each time Oh My Zsh is loaded, in which case,
# to know which specific one was loaded, run: echo $RANDOM_THEME
# See https://github.com/ohmyzsh/ohmyzsh/wiki/Themes
ZSH_THEME="robbyrussell"

# Set list of themes to pick from when loading at random
# Setting this variable when ZSH_THEME=random will cause zsh to load
# a theme from this variable instead of looking in $ZSH/themes/
# If set to an empty array, this variable will have no effect.
# ZSH_THEME_RANDOM_CANDIDATES=( "robbyrussell" "agnoster" )

# Uncomment the following line to use case-sensitive completion.
# CASE_SENSITIVE="true"

# Uncomment the following line to use hyphen-insensitive completion.
# Case-sensitive completion must be off. _ and - will be interchangeable.
# HYPHEN_INSENSITIVE="true"

# Uncomment one of the following lines to change the auto-update behavior
# zstyle ':omz:update' mode disabled  # disable automatic updates
# zstyle ':omz:update' mode auto      # update automatically without asking
# zstyle ':omz:update' mode reminder  # just remind me to update when it's time

# Uncomment the following line to change how often to auto-update (in days).
# zstyle ':omz:update' frequency 13

# Uncomment the following line if pasting URLs and other text is messed up.
# DISABLE_MAGIC_FUNCTIONS="true"

# Uncomment the following line to disable colors in ls.
# DISABLE_LS_COLORS="true"

# Uncomment the following line to disable auto-setting terminal title.
# DISABLE_AUTO_TITLE="true"

# Uncomment the following line to enable command auto-correction.
# ENABLE_CORRECTION="true"

# Uncomment the following line to display red dots whilst waiting for completion.
# You can also set it to another string to have that shown instead of the default red dots.
# e.g. COMPLETION_WAITING_DOTS="%F{yellow}waiting...%f"
# Caution: this setting can cause issues with multiline prompts in zsh < 5.7.1 (see #5765)
# COMPLETION_WAITING_DOTS="true"

# Uncomment the following line if you want to disable marking untracked files
# under VCS as dirty. This makes repository status check for large repositories
# much, much faster.
# DISABLE_UNTRACKED_FILES_DIRTY="true"

# Uncomment the following line if you want to change the command execution time
# stamp shown in the history command output.
# You can set one of the optional three formats:
# "mm/dd/yyyy"|"dd.mm.yyyy"|"yyyy-mm-dd"
# or set a custom format using the strftime function format specifications,
# see 'man strftime' for details.
# HIST_STAMPS="mm/dd/yyyy"

# Would you like to use another custom folder than $ZSH/custom?
# ZSH_CUSTOM=/path/to/new-custom-folder

# Which plugins would you like to load?
# Standard plugins can be found in $ZSH/plugins/
# Custom plugins may be added to $ZSH_CUSTOM/plugins/
# Example format: plugins=(rails git textmate ruby lighthouse)
# Add wisely, as too many plugins slow down shell startup.
plugins=(git)

source $ZSH/oh-my-zsh.sh

# User configuration

# export MANPATH="/usr/local/man:$MANPATH"

# You may need to manually set your language environment
# export LANG=en_US.UTF-8

# Preferred editor for local and remote sessions
# if [[ -n $SSH_CONNECTION ]]; then
#   export EDITOR='vim'
# else
#   export EDITOR='nvim'
# fi

# Compilation flags
# export ARCHFLAGS="-arch $(uname -m)"
# export OPENROUTER_API_KEY=""
# export ANTHROPIC_BASE_URL="https://openrouter.ai/api"
# export ANTHROPIC_AUTH_TOKEN="$OPENROUTER_API_KEY"
# export ANTHROPIC_API_KEY=""
export P24_GLAB_ROOT_FOLDER="$HOME/repos/p24"
export P24_ROOT_FOLDER="$P24_GLAB_ROOT_FOLDER/doktor24/frontends"
# export GITLAB_TOKEN=""
# export JIRA_EMAIL=""
# export JIRA_API_TOKEN=""

# Set personal aliases, overriding those provided by Oh My Zsh libs,
# plugins, and themes. Aliases can be placed here, though Oh My Zsh
# users are encouraged to define aliases within a top-level file in
# the $ZSH_CUSTOM folder, with .zsh extension. Examples:
# - $ZSH_CUSTOM/aliases.zsh
# - $ZSH_CUSTOM/macos.zsh
# For a full list of active aliases, run `alias`.
#
# Example aliases
# alias zshconfig="mate ~/.zshrc"
# alias ohmyzsh="mate ~/.oh-my-zsh"
alias brew='env PATH="${PATH//$(pyenv root)\/shims:/}" brew'

initP24SecretsShell() {
  op run --environment h4e7em6mxcdlsewgnzrqldjizi --no-masking -- $SHELL
}

p24SecretsRun() {
  op run --environment h4e7em6mxcdlsewgnzrqldjizi --no-masking -- "$@"
}


setPersonal_IDPlatform24Mac_GitConfig() {
  git config user.name "Arthur Vasconcelos"
  git config user.email vasconcelos.arthur@gmail.com
  git config user.signingkey "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIF9UkSRxB3d/rcLh0iz7tPSDdjJdobkgdyn+SBZDYkD"
  git config gpg.format ssh
  git config gpg.ssh.program "/Applications/1Password.app/Contents/MacOS/op-ssh-sign"
  git config commit.gpgsign true
}

initYarn() {
  nvm install
  nvm use
  corepack enable
}

initPnpm() {
  nvm install
  nvm use
  corepack enable
  corepack prepare pnpm@latest-9 --activate
}

pnpmInstall() {
  pnpm install --frozen-lockfile
}

# Function to create a Git commit with Jira task ID prefix from the current branch name
gcjp() {
  # Get the current branch name
  local branch_name
  branch_name=$(git symbolic-ref --short HEAD 2>/dev/null)

  if [ -z "$branch_name" ]; then
    echo "Not in a Git repository or on an unnamed branch."
    return 1
  fi

  # Extract the Jira task ID from the branch name
  local jira_id
  jira_id=$(echo "$branch_name" | sed -n -E 's/^[a-z]+\/(AX-[0-9]+)-.*$/\1/p')

  if [ -z "$jira_id" ]; then
    echo "No Jira task ID found in the branch name."
    return 1
  fi

  # Extract the commit message from the function argument
  local commit_message
  commit_message="$1"

  if [ -z "$commit_message" ]; then
    echo "Please provide a commit message."
    return 1
  fi

  # Create the commit with the Jira task ID prefix
  git commit -m "$jira_id: $commit_message"
}

export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"  # This loads nvm
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"  # This loads nvm bash_completion

eval "$(starship init zsh)"

# bun completions
[ -s "/Users/arthurvasconcelos/.bun/_bun" ] && source "/Users/arthurvasconcelos/.bun/_bun"

# bun
export BUN_INSTALL="$HOME/.bun"
export PATH="$BUN_INSTALL/bin:$PATH"

# overseer binary
export PATH="$HOME/bin:$PATH"

. "$HOME/.local/bin/env"
