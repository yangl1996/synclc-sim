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
            victims[node_index] = {"chain_growth": {"timestamp": timestamps, "height": heights}}

    dataset = {"victims": victims, "attackers": attackers, "first_chain_switch": first_chain_switch}
    with open(args.out, "w") as outfile:
        json.dump(dataset, outfile, sort_keys=True, indent=4)
