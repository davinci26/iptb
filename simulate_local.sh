#!/usr/bin/env bash

# ============== USAGE ==============
# Example Usage: ./simulate.sh 5 1 50 5 
# $1 Number of Nodes Start
# $2 Number of Nodes Increment
# $3 Number of Nodes End
# $4 File Size[Bytes] #E.g.10 MB file: 10485760 Bytes

for k in `seq $1 $2 $3`
do
    # Initalize network
    ./iptb init -n $k -f
    # Start nodes
    ./iptb start
    # Create Network Topology
    ./iptb make-topology
    # Create a random file and add it to Node 0
    head -c $4  </dev/urandom > file.txt
    file=$(./iptb run 0 ipfs add -Q file.txt)
    # Remove the file since we no longer need it
    rm file.txt
    # Make the simulation
    ./iptb dist -hash $file
    # Lets not burn the CPU and clean up after
    pkill ipfs
done
./bin/results_plotter.py -i results.json -size $4