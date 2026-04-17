FROM mcr.microsoft.com/devcontainers/go:1.25

RUN apt-get update \
	&& apt-get install -y --no-install-recommends pipx python3-venv \
	&& pipx install --global vpype \
	&& rm -rf /var/lib/apt/lists/*
