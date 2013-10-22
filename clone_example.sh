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

## How multiplying and limits work:
#
# sniffed traffic limited to L_PACKET_LIMIT ->
# -> HTTP requests ->
# -> multiplication by LISTENER_TRAFFIC_MULTIPLIER -> limit to L_HTTP_LIMIT ->
# -> multiplication by TRAFFIC_MULTIPLIER -> limit to REPLAY_RATE_LIMIT -> output

# multiply captured requests N times inside replay server
TRAFFIC_MULTIPLIER=1

# also multiplier, but inside the listener (not recommended)
LISTENER_TRAFFIC_MULTIPLIER=1

# who recieves cloned traffic
DEST_HOST_PORT=192.168.142.132:80

# limits on sniffing:
# IP packets per second
L_PACKET_LIMIT=100000
# HTTP requests per second (limited in listener)
L_HTTP_LIMIT=25000

# HTTP requests per second (limited in replay server; per replay server)
REPLAY_RATE_LIMIT=$((25000/$REPLAY_COUNT))

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
