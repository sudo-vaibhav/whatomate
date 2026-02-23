<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" alt="Zerodha Tech Badge" /></a>

# Whatomate

Modern, open-source WhatsApp Business Platform. Single binary app.

![Dashboard](docs/public/images/dashboard-light.png#gh-light-mode-only)
![Dashboard](docs/public/images/dashboard-dark.png#gh-dark-mode-only)

## Features

- **Multi-tenant Architecture**
  Support multiple organizations with isolated data and configurations.

- **Granular Roles & Permissions**
  Customizable roles with fine-grained permissions. Create custom roles, assign specific permissions per resource (users, contacts, templates, etc.), and control access at the action level (read, create, update, delete). Super admins can manage multiple organizations.

- **WhatsApp Cloud API Integration**
  Connect with Meta's WhatsApp Business API for messaging.

- **Real-time Chat**
  Live messaging with WebSocket support for instant communication.

- **Template Management**
  Create and manage message templates approved by Meta.

- **Bulk Campaigns**
  Send campaigns to multiple contacts with retry support for failed messages.

- **Chatbot Automation**
  Keyword-based auto-replies, conversation flows with branching logic, and AI-powered responses (OpenAI, Anthropic, Google).

- **Canned Responses**
  Pre-defined quick replies with slash commands (`/shortcut`) and dynamic placeholders.

- **Analytics Dashboard**
  Track messages, engagement, and campaign performance.

<details>
<summary>View more screenshots</summary>

![Dashboard](docs/public/images/dashboard-light.png#gh-light-mode-only)
![Dashboard](docs/public/images/dashboard-dark.png#gh-dark-mode-only)
![Chatbot](docs/public/images/chatbot-light.png#gh-light-mode-only)
![Chatbot](docs/public/images/chatbot-dark.png#gh-dark-mode-only)
![Agent Analytics](docs/public/images/agent-analytics-light.png#gh-light-mode-only)
![Agent Analytics](docs/public/images/agent-analytics-dark.png#gh-dark-mode-only)
![Conversation Flow Builder](docs/public/images/conversation-flow-light.png#gh-light-mode-only)
![Conversation Flow Builder](docs/public/images/conversation-flow-dark.png#gh-dark-mode-only)
![Templates](docs/public/images/11-templates.png)
![Campaigns](docs/public/images/13-campaigns.png)

</details>

## Installation

### Docker

The latest image is available on Docker Hub at [`shridh0r/whatomate:latest`](https://hub.docker.com/r/shridh0r/whatomate)

```bash
# Download compose file and sample config
curl -LO https://raw.githubusercontent.com/shridarpatil/whatomate/main/docker/docker-compose.yml
curl -LO https://raw.githubusercontent.com/shridarpatil/whatomate/main/config.example.toml

# Copy and edit config
cp config.example.toml config.toml

# Run services
docker compose up -d
```

Go to `http://localhost:8080` and login with `admin@admin.com` / `admin`

__________________

### Binary

Download the [latest release](https://github.com/shridarpatil/whatomate/releases) and extract the binary.

```bash
# Copy and edit config
cp config.example.toml config.toml

# Run with migrations
./whatomate server -migrate
```

Go to `http://localhost:8080` and login with `admin@admin.com` / `admin`

__________________

### Build from Source

```bash
git clone https://github.com/shridarpatil/whatomate.git
cd whatomate

# Production build (single binary with embedded frontend)
make build-prod
./whatomate server -migrate
```

See [configuration docs](https://shridarpatil.github.io/whatomate/getting-started/configuration/) for detailed setup options.

## Text-to-Speech (IVR Greetings)

Whatomate uses [Piper](https://github.com/rhasspy/piper) for offline text-to-speech generation. When admins type greeting text in the IVR flow editor, the server generates OGG/Opus audio files using Piper + `opusenc`. This is optional — you can also upload pre-recorded audio files directly.

### Install Piper

```bash
# Download Piper binary (Linux x86_64)
wget https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz
tar xf piper_linux_x86_64.tar.gz
sudo mv piper/piper /usr/local/bin/
```

### Install opusenc

```bash
# Debian/Ubuntu
sudo apt install opus-tools

# Fedora
sudo dnf install opus-tools
```

### Download a Voice Model

Piper voices are available at [huggingface.co/rhasspy/piper-voices](https://huggingface.co/rhasspy/piper-voices). Each voice has a `.onnx` model file and a `.onnx.json` config file — both are required.

**Choosing a voice:**
- Browse voices and listen to samples at [rhasspy.github.io/piper-samples](https://rhasspy.github.io/piper-samples/)
- Voices come in quality levels: `low`, `medium`, and `high` — `medium` is a good balance of quality and speed
- For US English, `en_US-lessac-medium` is recommended (~60MB)

```bash
mkdir -p /opt/piper/models

# Download model and config
wget https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx \
  -O /opt/piper/models/en_US-lessac-medium.onnx
wget https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json \
  -O /opt/piper/models/en_US-lessac-medium.onnx.json
```

### Configure

Add to your `config.toml`:

```toml
[tts]
piper_binary = "/usr/local/bin/piper"
piper_model = "/opt/piper/models/en_US-lessac-medium.onnx"
# opusenc_binary = "opusenc"  # defaults to finding in PATH
```

### Test

```bash
echo "Press 1 for sales, press 2 for support." | piper --model /opt/piper/models/en_US-lessac-medium.onnx --output_file test.wav
opusenc --bitrate 24 test.wav test.ogg
# Play: aplay test.wav  OR  ffplay test.ogg
```

Generated audio files are cached in the `audio_dir` (default: `./audio`) using a SHA256 hash of the text — same text always reuses the existing file.

## CLI Usage

```bash
./whatomate server              # API + 1 worker (default)
./whatomate server -workers=0   # API only
./whatomate worker -workers=4   # Workers only (for scaling)
./whatomate version             # Show version
```

## Developers

The backend is written in Go ([Fastglue](https://github.com/zerodha/fastglue)) and the frontend is Vue.js 3 with shadcn-vue.
- If you are interested in contributing, please read [CONTRIBUTING.md](./CONTRIBUTING.md) first.

```bash
# Development setup
make run-migrate    # Backend (port 8080)
cd frontend && npm run dev   # Frontend (port 3000)
```

## License

See [LICENSE](LICENSE) for details.
