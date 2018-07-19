#!/usr/bin/env bash

# ============== USAGE ==============
# Example Usage: ./test.sh 5 1 50 5 
# $1 Nodes Start
# $2 Nodes Increment
# $3 Nodes End
# $4 File Size[Bytes]
# TODO ADD $5 Connection Type flag -utp 

pkill ipfs
for k in `seq $1 $2 $3`
do
    ./iptb init -n $k -f
    ./iptb start
    # Add file to first node
    head -c $4  </dev/urandom > file.txt
    file=$(iptb run 0 ipfs add -Q file.txt)
    rm file.txt
    # Magic happens here
    ./iptb dist $file
    # Lets not burn the CPU
    pkill ipfs
done

# 10 MB file: 10485760