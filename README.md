# Nivi

Nivi is a Go-based, terminal-first AI assistant for NVIDIA-hosted models.

V1 is focused on a fast single-binary CLI for:

- interactive chat in the terminal
- one-shot prompts
- piping stdin into a prompt
- listing and switching available models

## Installation

Nivi V1 ships as a single native binary. Download the latest release for your platform from [GitHub Releases](https://github.com/LostWarrior/nivi/releases), place `nivi` on your `PATH`, then configure and run:

```bash
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

## Optional Environment Variables

- `NIVI_MODEL`
- `NIVI_BASE_URL`
- `NIVI_SYSTEM_PROMPT`
