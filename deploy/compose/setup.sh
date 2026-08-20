#!/usr/bin/env bash
set -euo pipefail

SOURCE_URL=${NORN_SOURCE_URL:-https://raw.githubusercontent.com/usenorn/norn/main/deploy/compose}
COMPOSE_FILE=compose.yaml
EXAMPLE_FILE=.env.example
ENV_FILE=.env
SELF_FILE=setup.sh

SELF=${BASH_SOURCE[0]:-}
if [ -f "$SELF" ]; then
	WORKDIR=$(CDPATH= cd -- "$(dirname -- "$SELF")" && pwd)
else
	WORKDIR=${NORN_DIR:-$PWD/norn}
	mkdir -p "$WORKDIR"
fi
cd "$WORKDIR"

say() { printf '  %s\n' "$*"; }
step() { printf '  %-28s' "$*"; }
done_() { printf 'done\n'; }
die() { printf '\nerror: %s\n' "$*" >&2; exit 1; }

usage() {
	cat <<'USAGE'
usage: ./setup.sh [command]

  install      configure and start Norn (default)
  start        start the services
  stop         stop the services, keeping data
  restart      restart the services
  upgrade      pull the configured release and restart into it
  status       show what is running
  logs [name]  follow logs, for one service or all of them
  uninstall    remove the containers, keeping every volume

  --domain <host>   answer the domain question without prompting
  --email <address> answer the certificate contact question
  --http            serve plain http instead of requesting a certificate
USAGE
}

have() { command -v "$1" >/dev/null 2>&1; }

version_at_least() {
	printf '%s\n%s\n' "$2" "$1" | sort -C -t. -k1,1n -k2,2n -k3,3n
}

require_docker() {
	have docker || die "docker is not installed: https://docs.docker.com/engine/install"
	docker info >/dev/null 2>&1 || die "the docker daemon is not reachable from this user"
	docker compose version >/dev/null 2>&1 || die "the docker compose plugin is not installed"

	local version
	version=$(docker compose version --short 2>/dev/null | tr -d 'v')
	version_at_least "$version" 2.23.0 ||
		die "docker compose $version is too old; 2.23 or newer is required"
}

fetch() {
	if [ -f "$1" ]; then
		return 0
	fi
	have curl || die "curl is not installed, and $1 is not in this directory"
	curl -fsSL -o "$1" "$SOURCE_URL/$1" || die "could not download $1 from $SOURCE_URL"
}

random_hex() {
	if have openssl; then
		openssl rand -hex 32
	else
		head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
	fi
}

random_base64() {
	if have openssl; then
		openssl rand -base64 32
	else
		head -c 32 /dev/urandom | base64 | tr -d '\n'
	fi
}

set_env() {
	local key=$1 value=$2 tmp
	tmp=$(mktemp)
	if grep -q "^${key}=" "$ENV_FILE"; then
		while IFS= read -r line || [ -n "$line" ]; do
			case $line in
			"${key}="*) printf '%s=%s\n' "$key" "$value" ;;
			*) printf '%s\n' "$line" ;;
			esac
		done <"$ENV_FILE" >"$tmp"
	else
		cat "$ENV_FILE" >"$tmp"
		printf '%s=%s\n' "$key" "$value" >>"$tmp"
	fi
	mv "$tmp" "$ENV_FILE"
	chmod 600 "$ENV_FILE"
}

ask() {
	local answer
	printf '  %-22s: ' "$1" >&2
	if [ -t 0 ]; then
		IFS= read -r answer || answer=""
	elif [ -r /dev/tty ]; then
		IFS= read -r answer </dev/tty || answer=""
	else
		die "no terminal to ask on; pass --domain and --email instead"
	fi
	printf '%s' "$answer"
}

valid_domain() {
	case $1 in
	"" | *[[:space:]]* | */* | *:*) return 1 ;;
	*.*) return 0 ;;
	*) return 1 ;;
	esac
}

configure() {
	if [ -f "$ENV_FILE" ]; then
		say "$ENV_FILE already exists, keeping it"
		return 0
	fi

	fetch "$EXAMPLE_FILE"

	while ! valid_domain "${DOMAIN:-}"; do
		if [ -n "${DOMAIN:-}" ]; then
			say "that is not a hostname, such as norn.example.com"
		fi
		DOMAIN=$(ask "Domain")
	done

	if [ "$SCHEME" = https ]; then
		while [ -z "${EMAIL:-}" ]; do
			EMAIL=$(ask "Email for TLS certs")
		done
	fi

	cp "$EXAMPLE_FILE" "$ENV_FILE"
	chmod 600 "$ENV_FILE"

	printf '\n'
	step "Generating secrets..."
	set_env POSTGRES_PASSWORD "$(random_hex)"
	set_env VALKEY_PASSWORD "$(random_hex)"
	set_env NORN_SECURITY_ENCRYPTION_KEY "$(random_base64)"
	done_

	step "Writing $ENV_FILE..."
	set_env NORN_APP_BASE_URL "$SCHEME://$DOMAIN"
	set_env ACME_EMAIL "${EMAIL:-admin@$DOMAIN}"
	if [ "$SCHEME" = http ]; then
		set_env NORN_SESSION_SECURE false
	fi
	done_
}

compose() { docker compose "$@"; }

start_services() {
	step "Pulling images..."
	compose pull --quiet >/dev/null 2>&1
	done_

	step "Starting services..."
	if ! compose up -d --wait >/dev/null 2>&1; then
		printf 'failed\n'
		compose ps -a
		die "a service did not start; inspect it with ./setup.sh logs"
	fi
	done_
}

base_url() { grep '^NORN_APP_BASE_URL=' "$ENV_FILE" | cut -d= -f2-; }

install() {
	printf '\n  Norn installer\n\n'
	say "Directory: $WORKDIR"
	printf '\n'
	require_docker
	fetch "$COMPOSE_FILE"
	fetch "$SELF_FILE"
	chmod +x "$SELF_FILE"
	configure
	start_services
	printf '\n'
	say "Norn is running at $(base_url)"
	say "Open it and create the first account."
	printf '\n'
	say "Invitations and password recovery need SMTP. Set NORN_SMTP_HOST and"
	say "NORN_SMTP_FROM_ADDRESS in $ENV_FILE, then run ./setup.sh restart."
	say "Every setting: https://docs.norn.so/self-hosting/environment-variables"
	printf '\n'
}

upgrade() {
	require_docker
	printf '\n  Norn upgrade\n\n'
	say "Back up before upgrading:"
	say "https://docs.norn.so/self-hosting/compose-operations#backups"
	printf '\n'
	start_services
	printf '\n'
	say "Norn is running at $(base_url)"
	printf '\n'
}

uninstall() {
	require_docker
	compose down
	printf '\n'
	say "The containers are gone and every volume is kept."
	say "To destroy the database, attachments and certificates as well:"
	say "docker compose down --volumes"
	printf '\n'
}

DOMAIN=${NORN_DOMAIN:-}
EMAIL=${ACME_EMAIL:-}
SCHEME=https
COMMAND=""
ARGS=()

value_of() {
	[ $# -ge 2 ] || die "$1 needs a value"
	printf '%s' "$2"
}

while [ $# -gt 0 ]; do
	case $1 in
	--domain)
		DOMAIN=$(value_of "$@")
		shift 2
		;;
	--email)
		EMAIL=$(value_of "$@")
		shift 2
		;;
	--http)
		SCHEME=http
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	-*)
		usage
		die "unknown option $1"
		;;
	*)
		if [ -z "$COMMAND" ]; then
			COMMAND=$1
		else
			ARGS+=("$1")
		fi
		shift
		;;
	esac
done

case ${COMMAND:-install} in
install) install ;;
start) require_docker && compose up -d --wait ;;
stop) require_docker && compose stop ;;
restart) require_docker && compose up -d --wait ;;
upgrade) upgrade ;;
status) require_docker && compose ps -a ;;
logs) require_docker && compose logs --follow --tail 100 ${ARGS[@]+"${ARGS[@]}"} ;;
uninstall) uninstall ;;
*)
	usage
	die "unknown command ${COMMAND}"
	;;
esac
