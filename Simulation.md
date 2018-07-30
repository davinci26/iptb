### Measure the performance of IPFS using IPTB

The simulation has two components:
- iptb make-topology: This creates a connection graph between the nodes (e.g. star topology, barbell topology). In the topology files empty lines and lines starting with # are disregarded. For non empty line the syntax is `origin:connection 1, connection 2 ...` where origin and connections are specified with their node ID.
- iptb dist -hash: The simulation here distributes a single file from node 0 to every other node in the network. Then it calculates the average time required to download the file, the standard deviation of the time, the maximum time, the minimum time, the duplicate blocks. The results are saved in a generated file called results.json. 


You can use local node and run it as follows:
```
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
```
If you want to simulate bad network conditions you need to use a docker type node. Depending on your docker permissions you may need to run these commands as root.
```
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
```
Before running the iptb dist command specify the following network parameters to emulate a bad network:
```
# add 50ms of latency to everything node 4 does
iptb set latency 50ms 4

# limit nodes 3-5 to 12Mbps (input parsing is bad here, i know)
iptb set bandwidth 12 [3-5]

# set a 6% packet loss on node 9
iptb set loss 6 9

# set latency jitter (+/-) of 7ms on nodes 0 through 9
iptb set jitter 7ms [0-9]

```
