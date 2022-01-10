now=`date '+%s'`
startTime=`expr $now + 5`
./synclc-sim -local 1 -global 2 -lottery 0 -parallel 4 -peers localhost:10000,localhost:10001 -port 9000 -seed 1 -start $startTime &> victim.txt &
./synclc-sim -lottery 0.2 -parallel 4 -port 10000 -seed 2 -start $startTime &> honest.txt &
./synclc-sim -lottery 1 -parallel 4 -port 10001 -seed 3 -start $startTime -ri 0.1s -attack &> attacker.txt &
