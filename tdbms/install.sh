#!/bin/bash

# This script performs the installation of the Tanabata DBMS server

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit 1
fi

cd "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

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
chmod 2775 /var/log/tanabata

if [ -d ../build ]; then
  rm -r ../build/*
else
  mkdir ../build
  if [ -d ../build ]; then
    echo "FATAL: failed to create build directory"
    exit 1
  fi
fi
if ! (cmake -S .. -B ../build && cmake --build ../build --target tdbms); then
  echo "FATAL: failed to build TDBMS server"
  exit 1
fi
mv -f ../build/tdbms /usr/bin/
chown 0:0 /usr/bin/tdbms
chmod 0755 /usr/bin/tdbms

if ! cp ./tdbms.service /etc/systemd/system/; then
  echo "FATAL: failed to copy 'tdbms.service' to '/etc/systemd/system'"
  exit 1
fi
chown 0:0 /etc/systemd/system/tdbms.service
chmod 0644 /etc/systemd/system/tdbms.service

echo "TDBMS server successfully installed."
echo "Start it with 'systemctl start tdbms'"
