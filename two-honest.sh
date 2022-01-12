now=`date '+%s'`
startTime=`expr $now + 7`

#relay
./synclc-sim -local 10 -global 10 -lottery 0 -parallel 4 -port 9002 -start $startTime &> relay.txt &

#attacker
./synclc-sim -lottery 0.1 -parallel 4 -port 10000 -start $startTime -attack -seed 1 &> attacker.txt &

#two victims
mm-delay 50 mm-link fast.mahi fast.mahi --uplink-queue=pie --uplink-queue-args="qdelay_ref=20, packets=200, max_burst=150" --downlink-queue=pie --downlink-queue-args="qdelay_ref=20, packets=200, max_burst=150" -- ./synclc-sim -local 1 -global 1 -lottery 0.1 -parallel 4 -peers 172.16.232.130:9002,172.16.232.130:10000 -port 9000 -start $startTime -output "victim1" &> victim1.txt &

mm-delay 50 mm-link fast.mahi fast.mahi --uplink-queue=pie --uplink-queue-args="qdelay_ref=20, packets=200, max_burst=150" --downlink-queue=pie --downlink-queue-args="qdelay_ref=20, packets=200, max_burst=150" -- ./synclc-sim -local 1 -global 1 -lottery 0.1 -parallel 4 -peers 172.16.232.130:9002,172.16.232.130:10000 -port 9001 -start $startTime -output "victim2" &> victim2.txt &

