#!/usr/bin/python

import math
import os
import sys
import signal
from argparse import ArgumentParser
from multiprocessing import Process
from subprocess import PIPE, Popen
from time import sleep, time

from mininet.cli import CLI
from mininet.link import Link, TCIntf, TCLink
from mininet.log import debug, info, lg
from mininet.net import Mininet
from mininet.node import CPULimitedHost
from mininet.topo import Topo
from mininet.util import dumpNodeConnections

parser = ArgumentParser(description="SyncLC experiments")
parser.add_argument('--bw-victim',
                    type=float,
                    help="Bandwidth of victim links (Mb/s)",
                    default=10)

parser.add_argument('--bw-attacker',
                    type=float,
                    help="Bandwidth of attacker links (Mb/s)",
                    default=1000)

parser.add_argument('--local-cap',
                    '-L',
                    type=int,
                    help="Local cap",
                    default=1)

parser.add_argument('--global-cap',
                    '-G',
                    type=int,
                    help="Global cap",
                    default=1)

parser.add_argument('--num-victims',
                    '-V',
                    type=int,
                    help="Number of victims",
                    default=3)

parser.add_argument('--num-attackers',
                    '-A',
                    type=int,
                    help="Number of attackers",
                    default=1)

parser.add_argument('--rule',
                    type=str,
                    help="Download rule",
                    choices=['freshest', 'longest'],
                    default='longest')

parser.add_argument('--victim-lottery',
                    '-M',
                    type=float,
                    help="Chance of winning a lottery per victim",
                    default=0.1)

parser.add_argument('--attacker-lottery',
                    '-B',
                    type=float,
                    help="Chance of winning a lottery for the attacker",
                    default=0.1)

parser.add_argument('--delay',
                    type=int,
                    help="Link propagation delay (ms) on each link to the switch",
                    default=25)

parser.add_argument('--maxq',
                    type=int,
                    help="Max buffer size of network interface in packets",
                    default=300)

parser.add_argument('--cong',
                    help="Congestion control algorithm to use",
                    default="reno")

parser.add_argument('--no-egress-limit',
                    help="Do not limit egress bandwidth of victims",
                    default=False,
                    action="store_true")

parser.add_argument('--block-size',
                    help="Block size in bytes",
                    default=100000,
                    type=int)

parser.add_argument('--duration',
                    help="Duration of the experiment in seconds",
                    default=1800,
                    type=int)

# Expt parameters
args = parser.parse_args()

class BBTopo(Topo):

    def __init__(self, victims=2, attackers=1):
        super(BBTopo, self).__init__()

        s0 = self.addSwitch('s0')

        for i in range(victims):
            v = self.addHost('v{}'.format(i))
            # use PIE at the switch to avoid tuning the buffer size
            self.addLink(v, s0, bw=args.bw_victim, delay="{}ms".format(
                args.delay), max_queue_size=args.maxq)

        for i in range(attackers):
            a = self.addHost('a{}'.format(i))
            self.addLink(a, s0, bw=args.bw_attacker, delay="{}ms".format(
                args.delay), max_queue_size=args.maxq)

        return


def start_victim(net, victim_idx, num_victim, num_adv, at_unix, local_cap, global_cap, lottery, download_rule, block_size):
    peers = []
    for i in range(num_victim):
        if i <= victim_idx:
            continue
        peer = net.getNodeByName('v{}'.format(i))
        peers.append("{}:8000".format(peer.IP()))

    for i in range(num_adv):
        peer = net.getNodeByName('a{}'.format(i))
        peers.append("{}:8000".format(peer.IP()))

    v = net.getNodeByName('v{}'.format(victim_idx))
    if args.no_egress_limit:
        v.intf().config(bw=1000, smooth_change = False) 
    output_prefix = "victim_{}".format(victim_idx)
    proc = v.popen("./synclc-sim -local {} -global {} -lottery {} -parallel 4 -start {} -peers {} -output {} -rule {} -size {} &> {}.log".format(local_cap, global_cap, lottery, at_unix, ','.join(peers), output_prefix, download_rule, block_size, output_prefix), shell=True)
    mon = v.popen("sudo bmon -o format:fmt='$(element:name) rxbytes=$(attr:rx:bytes) txbytes=$(attr:tx:bytes)\n' -p '{}' &> {}-traffic.txt".format(v.intf(None).name, output_prefix), shell=True)
    return proc

def start_attacker(net, adv_idx, at_unix, lottery, download_rule, block_size):
    a = net.getNodeByName('a{}'.format(adv_idx))
    output_prefix = "attacker_{}".format(adv_idx)
    proc = a.popen("./synclc-sim -lottery {} -parallel 4 -start {} -rule {} -attack -seed 42 -size {} &> {}.log".format(lottery, at_unix, download_rule, block_size, output_prefix), shell=True)
    return proc


if __name__ == "__main__":
    os.system("sysctl -w net.ipv4.tcp_congestion_control=%s" % args.cong)
    topo = BBTopo(victims=args.num_victims, attackers=args.num_attackers)
    net = Mininet(topo=topo, link=TCLink)
    net.start()

    def sigint_handler(sig, frame):
        print("SIGINT captured, cleaning up")
        os.system("sudo pkill synclc-sim")
        os.system("sudo pkill bmon")
        net.stop()
        os.system("mn -c")
        sys.exit(0)
    signal.signal(signal.SIGINT, sigint_handler)

    # This performs a basic all pairs ping test.
    #net.pingAll()

    start_at = int(time()) + 5
    for i in range(args.num_victims):
        start_victim(net, i, args.num_victims, args.num_attackers, start_at, args.local_cap, args.global_cap, args.victim_lottery, args.rule, args.block_size)
    for i in range(args.num_attackers):
        start_attacker(net, i, start_at, args.attacker_lottery, args.rule, args.block_size)

    sleep(args.duration)
    sigint_handler(None, None)
