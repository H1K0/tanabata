#!/bin/bash

# This script performs the installation of Tanabata web server

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit 1
fi

cd "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

../tdbms/install.sh || exit 1

usermod -a -G tanabata www-data

if [ ! -d /var/lib/tanabata/tweb ]; then
  mkdir /var/lib/tanabata/tweb
  if [ ! -d /var/lib/tanabata/tweb ]; then
    echo "FATAL: failed to create directory '/var/lib/tanabata/tweb'"
    exit 1
  fi
fi
chown 42776:42776 /var/lib/tanabata/tweb
chmod 2755 /var/lib/tanabata/tweb

if [ -d ../build ]; then
  rm -r ../build/*
else
  mkdir ../build
  if [ -d ../build ]; then
    echo "FATAL: failed to create build directory"
    exit 1
  fi
fi
cd ./server
echo "Building Tweb server..."
if ! go build -o ../build; then
  echo "FATAL: failed to build Tweb server"
  exit 1
fi
cd ..
mv -f ../build/tweb /usr/bin/
chown 0:0 /usr/bin/tweb
chmod 0755 /usr/bin/tweb

if ! cp ./tweb.service /etc/systemd/system/; then
  echo "FATAL: failed to copy 'tweb.service' to '/etc/systemd/system'"
  exit 1
fi
chown 0:0 /etc/systemd/system/tweb.service
chmod 0644 /etc/systemd/system/tweb.service

if ! cp -r ./public/* /srv/www/tanabata/; then
  echo "FATAL: failed to copy public files to '/srv/www/tanabata'"
  exit 1
fi

echo "Tweb server successfully installed."
echo "Start it with 'systemctl start tweb'"
