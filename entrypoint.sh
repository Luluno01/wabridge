#!/bin/sh
set -e
umask 077
exec wabridge "$@"
