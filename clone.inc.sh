# general functions for traffic cloning setup

function kill_old_replays() {
    if [ -f $FILE_WITH_PIDS ]
    then
        set +e
        cat $FILE_WITH_PIDS | xargs kill
        set -e
    fi
}

function start_replays() {
    echo "staring #<$REPLAY_COUNT> replay servers with #<$THREADS> threads and backlog of size #<$BACKLOG>"

    rm -f $FILE_WITH_PIDS

    for (( i=0; i <= $(($REPLAY_COUNT-1)); i++ ))
    do
        echo "start replay server $i"
        python replay.py $STATSD \
            --socket="$UNIX_SOCKET_PATH.$i" \
            --threads $THREADS --backlog $BACKLOG \
            --multiplier $TRAFFIC_MULTIPLIER \
            --upstream "$DEST_HOST_PORT" $EXTRA_REPLAY_PARAMS > /dev/null &
        REPLAY_PID=$!
        echo $REPLAY_PID >>$FILE_WITH_PIDS
        echo "pid = $REPLAY_PID started"
    done

    ps ax | grep "gor replay" | wc -l
}

function start_listener() {
    echo "starting listener"
    GOMAXPROCS=6 ./gor listen $STATSD \
        -i $IFACE --pcap-filter "$PCAP_FILTER" \
        -r "$UNIX_SOCKET_PATH|$L_HTTP_LIMIT" \
        --pool-size $REPLAY_COUNT --packet-limit $L_PACKET_LIMIT \
        --multiply $LISTENER_TRAFFIC_MULTIPLIER \
        --die-if-replay-server-is-unreachable --dgram
}

kill_old_replays
start_replays
start_listener
