#!/usr/bin/env bash
# Filter traffic Docker publishes, which UFW does not.
#
# **Why this exists.** A reviewer asked, on 2026-10-08: "Did you set up
# Docker iptables? Docker likes to ignore firewalls on a host." He was
# right, and the answer was no.
#
# UFW on this box is correct on paper: default deny inbound, allowing
# only 22, 80 and 443. But Docker inserts its own rules ahead of UFW's
# chain for anything it publishes, so a published container port is
# reachable from the internet whatever UFW says. The DOCKER-USER chain
# is the hook Docker leaves for exactly this, and it was empty.
#
# Nothing was exposed, because nothing but Caddy publishes a port:
# postgres, api, web, sidecar, ollama and minio all use
# `ports: !override []` in docker-compose.prod.yml. But that is an
# argument from "we did not publish anything", not from a control. The
# first time somebody adds a `ports:` line to debug something, it is on
# the public internet immediately and UFW reports that all is well.
#
# So: default deny for anything arriving from the public interface
# towards a container, with 80 and 443 allowed because that is the
# site. Published-by-accident ports are now dropped rather than served.
#
# DOCKER-USER is traversed for FORWARDed packets only, so this cannot
# affect SSH, which arrives on INPUT. Locking yourself out with this
# script is not possible.
set -euo pipefail

PUBLIC_IF="${PUBLIC_IF:-$(ip -o -4 route show default | awk '{print $5; exit}')}"
[ -n "$PUBLIC_IF" ] || { echo "cannot determine the public interface" >&2; exit 1; }

apply() {
  local ipt="$1"
  # The chain exists because Docker creates it. If Docker has not
  # started yet there is nothing to filter, and failing here would
  # leave the box less protected than doing nothing.
  "$ipt" -L DOCKER-USER -n >/dev/null 2>&1 || return 0

  # Idempotent: rebuilt from scratch each time rather than appended to,
  # so running this twice does not stack duplicate rules.
  "$ipt" -F DOCKER-USER

  # Anything not arriving from the public interface is container to
  # container, or loopback, and is none of this chain's business.
  "$ipt" -A DOCKER-USER ! -i "$PUBLIC_IF" -j RETURN

  # Replies to connections the box itself opened.
  "$ipt" -A DOCKER-USER -m conntrack --ctstate RELATED,ESTABLISHED -j RETURN

  # The site. Caddy terminates TLS and is the only thing meant to be
  # reachable; 443/udp is for HTTP/3 if it is ever published.
  "$ipt" -A DOCKER-USER -i "$PUBLIC_IF" -p tcp --dport 80  -j RETURN
  "$ipt" -A DOCKER-USER -i "$PUBLIC_IF" -p tcp --dport 443 -j RETURN
  "$ipt" -A DOCKER-USER -i "$PUBLIC_IF" -p udp --dport 443 -j RETURN

  # Everything else from the internet to a container.
  "$ipt" -A DOCKER-USER -i "$PUBLIC_IF" -j DROP
}

apply iptables
# IPv6 too. The site has an AAAA record, so a v4-only rule would leave
# the whole thing open over v6, which is the kind of half-measure that
# reads as protection and is not.
apply ip6tables

echo "DOCKER-USER filtered on ${PUBLIC_IF}: 80 and 443 allowed, the rest dropped"
