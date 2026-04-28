# Nivi

Nivi is a Go-based, terminal-first AI assistant for NVIDIA-hosted models.

V1 is focused on a fast single-binary CLI for:

- interactive chat in the terminal
- one-shot prompts
- piping stdin into a prompt
- listing and switching available models

## Installation

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/LostWarrior/nivi/main/scripts/install.sh | sh
```

The installer downloads the matching release asset for macOS or Linux (`amd64` and `arm64`) and installs `nivi` to `~/.local/bin` by default.

Pin a release or change the install directory:

```bash
NIVI_VERSION=v0.1.0 sh -c "$(curl -fsSL https://raw.githubusercontent.com/LostWarrior/nivi/main/scripts/install.sh)"
NIVI_INSTALL_DIR="$HOME/bin" sh -c "$(curl -fsSL https://raw.githubusercontent.com/LostWarrior/nivi/main/scripts/install.sh)"
```

Then configure and run:

```bash
export PATH="$HOME/.local/bin:$PATH"
echo 'export NVIDIA_API_KEY="nvapi-your-key-here"' >> ~/.zshrc
source ~/.zshrc
nivi
```

Alternative:

- Browse release assets manually at [GitHub Releases](https://github.com/LostWarrior/nivi/releases)

## Usage

- `nivi` starts an interactive chat session
- `nivi "explain this stack trace"` runs a one-shot prompt
- `cat file.py | nivi "review this"` sends stdin with a prompt
- `nivi models` lists models available to the current API key
- `nivi -m <model-id>` selects a model for the current command
- `/model` switches models during a chat session
- `nivi version` prints the installed version

## Optional Environment Variables

- `NIVI_MODEL`
- `NIVI_BASE_URL`
- `NIVI_SYSTEM_PROMPT`
