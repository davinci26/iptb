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
    ./iptb init -n $k -f --type=docker
    # Start nodes
    ./iptb start
    # Create Network Topology
    ./iptb make-topology
    # Create random file
    head -c $4  </dev/urandom > file.txt
    # Push the file to docker container
    dockID=$(cat ~/testbed/0/dockerID)
    docker cp file.txt $dockID:file.txt
    # Add it to Node 0
    file=$(./iptb run 0 ipfs add -Q file.txt)
    rm file.txt
    # Simulate
    ./iptb dist -hash $file
    pkill dockerd
done
# Plot results
./bin/results_plotter.py -i results.json -size $4