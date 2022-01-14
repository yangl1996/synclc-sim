#!/usr/bin/python

import math
import os
import sys
import signal
from argparse import ArgumentParser
from time import sleep, time
import glob
import re
import json
from dateutil.parser import *

parser = ArgumentParser(description="SyncLC data processing")
parser.add_argument('--dir',
                    type=str,
                    help="Directory of the result files",
                    default=".")

parser.add_argument('--out',
                    type=str,
                    help="File for output",
                    default="output.json")

# Expt parameters
args = parser.parse_args()

victims = {}
attackers = {}

file_index_re = re.compile(r"(attacker|victim)_([0-9]+)")
chain_growth_re = re.compile(r"switched to block [a-z0-9]+ height ([0-9]+) at time ([0-9]+)")
attack_point_keyword = "downloaded invalid block"
if __name__ == "__main__":
    files=glob.glob(args.dir+"/*.log")
    first_chain_switch = None
    for filepath in files:
        filename = filepath.split('/')[-1]
        node_info = file_index_re.search(filename)
        node_index = int(node_info.group(2))
        node_type = node_info.group(1)
        print("processing {} {}".format(node_type, node_index))
        if 'victim_' in filename:
            timestamps = []
            heights = []
            attack_starts = []
            attack_ends = []
            last_attack = None
            with open(filepath) as f:
                for line in f:
                    result = chain_growth_re.search(line)
                    if not result is None:
                        time = int(result.group(2))
                        height = int(result.group(1))
                        timestamps.append(time)
                        heights.append(height)
                        if first_chain_switch is None or first_chain_switch > time:
                            first_chain_switch = time
                    # attack
                    if attack_point_keyword in line:
                        time = parse(line[0:19]).timestamp()
                        if last_attack == None or time-last_attack > 2.0:
                            attack_starts.append(time*1000000)
                            if not last_attack is None:
                                attack_ends.append(last_attack*1000000)
                        last_attack = time
                if not last_attack is None:
                    attack_ends.append(last_attack*1000000)
            victims[node_index] = {"chain_growth": {"timestamp": timestamps, "height": heights}, "under_spam": {"start_timestamp": attack_starts, "end_timestamp": attack_ends}}

    dataset = {"victims": victims, "attackers": attackers, "first_chain_switch": first_chain_switch}
    with open(args.out, "w") as outfile:
        json.dump(dataset, outfile, sort_keys=True, indent=4)
