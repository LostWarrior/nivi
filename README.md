# Nivi

Nivi is a terminal-first AI assistant for NVIDIA-hosted models

## Installation

Install, configure, and run:

```bash
# installer command will be added once the public release URL is finalized
echo 'export NVIDIA_API_KEY="nvapi-your-key-here"' >> ~/.zshrc
source ~/.zshrc
nivi
```

## Help Commands

- `nivi` starts an interactive REPL
- `nivi` is the primary documented command and enters chat directly
- `nivi "explain this stack trace"` runs a one-shot prompt
- `cat file.py | nivi "review this"` accepts stdin
- `nivi models` lists models available to the current API key
- `nivi -m <model-id>` overrides the default model
- `/model` switches models during a chat session

## Optional Environment Variables

- `NIVI_MODEL`
- `NIVI_BASE_URL`
- `NIVI_SYSTEM_PROMPT`
