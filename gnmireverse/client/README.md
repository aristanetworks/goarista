gnmireverse client is a process that subscribes to a gNMI target
(typically running on the same host as this process) and forwards the
responses to a gnmireverse server. This can be used to reverse the
dial direction of a gNMI Subscribe.

Run the program with the flag `-h` to see documentation on each of the
options.

The client requires a gNMI target address to connect to and a
gNMIReverse server to send the subscription results.

For example, on an Arista EOS device the client can be configured in the CLI:

```
daemon gnmireverse
   exec /mnt/flash/gnmireverse_client -username USER -password PASS # authenticate locally
   -target_addr=mgmt/127.0.0.1:6030 # default address of gNMI server, listening in mgmt VRF
   -collector_addr=mgmt/1.2.3.4:6000 # address of gNMIReverse server, connecting through mgmt VRF
   -target_value=device1 # Include a name for this device
   -sample interfaces/interface/state/counters@30s # stream interface counters sampled every 30 seconds
   -subscribe network-instances # stream changes as they happen to network-instances config and state
   no shutdown
```
