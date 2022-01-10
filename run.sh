now=`date '+%s'`
startTime=`expr $now + 5`
# victim
mm-delay 50 mm-link slow.mahi slow.mahi --uplink-queue=pie --uplink-queue-args="qdelay_ref=20, packets=200, max_burst=150" --downlink-queue=pie --downlink-queue-args="qdelay_ref=20, packets=200, max_burst=150" -- ./synclc-sim -inflight 10 -lottery 0 -parallel 4 -peers 172.16.232.130:10000,172.16.232.130:10001,172.16.232.130:10002,172.16.232.130:10003 -port 9000 -seed 1 -start $startTime &> victim.txt &
./synclc-sim -lottery 0.2 -parallel 4 -port 10000 -seed 2 -start $startTime &> honest.txt &
./synclc-sim -lottery 1.0 -ri 0.1s -parallel 4 -port 10001 -seed 3 -start $startTime -attack  &> attacker1.txt &
./synclc-sim -lottery 1.0 -ri 0.1s -parallel 4 -port 10002 -seed 4 -start $startTime -attack  &> attacker2.txt &
./synclc-sim -lottery 1.0 -ri 0.1s -parallel 4 -port 10003 -seed 5 -start $startTime -attack  &> attacker3.txt &
