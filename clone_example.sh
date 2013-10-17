#!/bin/bash -e
#
# ./clone_example.sh
#
#

# [ cmdline parameters ] ============================================
#
# number of replay servers (12 seems good for 5..15K RPS)
REPLAY_COUNT=12
# number of greenlet threads per replay server
THREADS=60

# string to insert in unix socket names to avoid conflicts with
# other traffic cloning projects
PROJECT=logger
# base path for unix sockets (".#socket_number" suffix will be appended)
UNIX_SOCKET_PATH=/tmp/gor.$PROJECT
# this file contains pids of all replay servers, one per line
FILE_WITH_PIDS=$PROJECT.pids

# [ traffic source and destination, limits ] ==========================

# where is the traffic
IFACE=eth1

# what traffic should we clone
PCAP_FILTER='(dst net 192.168.7.96/27) && ((tcp dst port 80) || (tcp dst portrange 9090-9100))'

# multiply captured requests N times
TRAFFIC_MULTIPLIER=1

# who recieves cloned traffic
DEST_HOST_PORT=192.168.142.132:80

# limits on sniffing:
# IP packets per second
L_PACKET_LIMIT=100000
# HTTP requests per second
L_HTTP_LIMIT=25000

# extra params to pass to replay server
EXTRA_REPLAY_PARAMS=


# [ misc settings ] ==================================================
#
# size of backlog queue in the replay server, in requests
# (when it's full, requests are dropped)
BACKLOG=8000
# statsd host
STATSD=--statsd=192.168.142.31:8125

source clone.inc.sh