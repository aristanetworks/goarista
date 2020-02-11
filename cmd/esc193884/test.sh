#!/bin/bash

set -x

Cli -c "enable
config
interface Ethernet18/1
no channel-group 801 mode active"
sleep 1

/mnt/flash/esc193884 -addr localhost:6042 -username admin -compression= > /mnt/flash/test1.txt 2>&1 &
pid1=$!
sleep 3
/mnt/flash/esc193884 -addr localhost:6042 -username admin -compression= > /mnt/flash/test2.txt 2>&1 &
pid2=$!
sleep 3
/mnt/flash/esc193884 -addr localhost:6042 -username admin -compression= > /mnt/flash/test3.txt 2>&1 &
pid3=$!

# sleep between 5-20 seconds
sleeptime=$RANDOM
let "sleeptime %= 15"
let "sleeptime += 5"
sleep $sleeptime

Cli -c "enable
config
interface Ethernet18/1
channel-group 801 mode active"

# sleep long enough for each subscriber to receive notifs about Ethernet18/1
sleep 25
# Tell subscribers to stop
kill $pid1 $pid2 $pid3

if ! wait $pid1
then
    cat /mnt/flash/test1.txt
    exit 1
fi
if ! wait $pid2 
then
    cat /mnt/flash/test2.txt
    exit 1
fi
if ! wait $pid3 
then
    cat /mnt/flash/test3.txt
    exit 1
fi
echo all good
