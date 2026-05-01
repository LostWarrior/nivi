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

Then configure and run:

```bash
export PATH="$HOME/.local/bin:$PATH"
echo 'export NVIDIA_API_KEY="nvapi-your-key-here"' >> ~/.zshrc
source ~/.zshrc
nivi
```
## Usage

- `nivi` starts an interactive chat session
- `nivi "explain this stack trace"` runs a one-shot prompt
- `cat file.py | nivi "review this"` sends stdin with a prompt
- `nivi models` lists models available to the current API key
- `nivi -m <model-id>` selects a model for the current command
- `/model` switches models during a chat session
- `nivi doctor` to check the setup
- `nivi version` prints the installed version

## Optional Environment Variables

- `NIVI_MODEL`
- `NIVI_BASE_URL`
- `NIVI_SYSTEM_PROMPT`
