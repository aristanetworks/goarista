The gNMIReverse gRPC service that reverses the direction of the dial
for gNMI Subscriptions.

[gNMI](https://github.com/openconfig/reference/tree/master/rpc/gnmi)
is a "dial-in" service. This means that a telemetry collector must
make the connection to the gNMI target (such as an ethernet
switch). This approach can cause issues in some deployments, such as
when the gNMI target is behind a NAT gateway.

A gNMIReverse client can be run alongside the gNMI target and then
"dial-out" to a gNMIReverse server to send streaming data.

An example gNMIReverse client and server program are provided in the
client and server directories.
