#!/bin/bash

# This script performs the installation of the TDBMS server

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit 1
fi

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

getent group tanabata &>/dev/null || groupadd -g 42776 tanabata
id tanabata &>/dev/null || useradd -u 42776 -g 42776 tanabata
if [ ! "$(id -nG 42776 | grep -w tanabata)" ]; then
  echo "FATAL: failed to create user and group 'tanabata'"
  exit 1
fi

if [ ! -d /etc/tanabata ]; then
  mkdir /etc/tanabata
  if [ ! -d /etc/tanabata ]; then
    echo "FATAL: failed to create directory '/etc/tanabata'"
    exit 1
  fi
fi
chown 42776:42776 /etc/tanabata
chmod 2755 /etc/tanabata

if [ ! -d /var/lib/tanabata ]; then
  mkdir /var/lib/tanabata
  if [ ! -d /var/lib/tanabata ]; then
    echo "FATAL: failed to create directory '/var/lib/tanabata'"
    exit 1
  fi
fi
chown 42776:42776 /var/lib/tanabata
chmod 2755 /var/lib/tanabata

if [ ! -d /var/lib/tanabata/tdbms ]; then
  mkdir /var/lib/tanabata/tdbms
  if [ ! -d /var/lib/tanabata/tdbms ]; then
    echo "FATAL: failed to create directory '/var/lib/tanabata/tdbms'"
    exit 1
  fi
fi
chown 42776:42776 /var/lib/tanabata/tdbms
chmod 2755 /var/lib/tanabata/tdbms

if [ ! -d /var/log/tanabata ]; then
  mkdir /var/log/tanabata
  if [ ! -d /var/log/tanabata ]; then
    echo "FATAL: failed to create directory '/var/log/tanabata'"
    exit 1
  fi
fi
chown 42776:42776 /var/log/tanabata
chmod 2755 /var/log/tanabata

if [ ! -d "$SCRIPT_DIR/../build" ]; then
  mkdir "$SCRIPT_DIR/../build"
fi
if ! (cmake -S "$SCRIPT_DIR/.." -B "$SCRIPT_DIR/../build" && cmake --build "$SCRIPT_DIR/../build" --target tdbms); then
  echo "FATAL: failed to build TDBMS server"
  exit 1
fi
mv -f "$SCRIPT_DIR/../build/tdbms" /usr/bin/
chown 0:0 /usr/bin/tdbms
chmod 0755 /usr/bin/tdbms

if ! cp "$SCRIPT_DIR/tdbms.service" /etc/systemd/system/; then
  echo "FATAL: sailed to copy 'tdbms.service' to '/etc/systemd/system'"
fi
chown 0:0 /etc/systemd/system/tdbms.service
chmod 0644 /etc/systemd/system/tdbms.service

echo "TDBMS server successfully installed."
echo "Start it with 'systemctl start tdbms'"