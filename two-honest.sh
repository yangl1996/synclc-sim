now=`date '+%s'`
startTime=`expr $now + 5`

#two victims
mm-delay 50 mm-link fast.mahi fast.mahi --uplink-queue=pie --uplink-queue-args="qdelay_ref=20, packets=200, max_burst=150" --downlink-queue=pie --downlink-queue-args="qdelay_ref=20, packets=200, max_burst=150" -- ./synclc-sim -local 1 -global 2 -lottery 0.1 -parallel 4 -peers 172.16.232.130:9002,172.16.232.130:10000 -port 9000 -seed 1 -start $startTime &> victim1.txt &
mm-delay 50 mm-link fast.mahi fast.mahi --uplink-queue=pie --uplink-queue-args="qdelay_ref=20, packets=200, max_burst=150" --downlink-queue=pie --downlink-queue-args="qdelay_ref=20, packets=200, max_burst=150" -- ./synclc-sim -local 1 -global 2 -lottery 0.1 -parallel 4 -peers 172.16.232.130:9002,172.16.232.130:10001 -port 9001 -seed 2 -start $startTime &> victim2.txt &

#relay
./synclc-sim -local 10 -global 10 -lottery 0 -parallel 4 -port 9002 -seed 3 -start $startTime &> relay.txt &

#attackers for victims 1, 2
./synclc-sim -lottery 0.01 -parallel 4 -port 10000 -seed 4 -start $startTime -attack &> attacker1.txt &
./synclc-sim -lottery 0.01 -parallel 4 -port 10001 -seed 5 -start $startTime -attack &> attacker2.txt &
