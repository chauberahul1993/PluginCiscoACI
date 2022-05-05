#!/bin/bash

trap "echo TERM" TERM
trap "echo HUP" HUP
trap "echo INT" INT
trap "echo QUIT" QUIT
trap "echo USR1; sleep 2; exit 0" USR1
trap "echo USR2" USR2

ps aux
tail -f /dev/null