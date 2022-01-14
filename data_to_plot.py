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
parser.add_argument('--input',
                    type=str,
                    help="Input JSON data file",
                    default="output.json")
parser.add_argument('--output',
                    type=str,
                    help="Output prefix",
                    default="")

# Expt parameters
args = parser.parse_args()

if __name__ == "__main__":
    with open(args.input) as f:
        d = json.load(f)
        first_chain_switch = d['first_chain_switch']
        tot_chain_growth_rates = 0.0
        for k in d['victims']:
            height = d['victims'][k]['chain_growth']['height']
            timestamp = d['victims'][k]['chain_growth']['timestamp']
            l = len(height)
            x = [(t-first_chain_switch)/1000000.0 for t in timestamp]
            with open(args.output+"chain_growth_victim_{}.txt".format(k), 'w') as outfile:
                outfile.write("time height\n")
                for i in range(l):
                    outfile.write("{} {}\n".format(x[i], height[i]))

            timePeriods = d['victims'][k]['under_spam']
            collect = []
            for i in range(len(timePeriods['start_timestamp'])):
                collect.append('{}/{}'.format((float(timePeriods['start_timestamp'][i])-first_chain_switch)/1000000.0, (float(timePeriods['end_timestamp'][i])-first_chain_switch)/1000000.0))
            print("victim {} attack timespans: {}".format(k, ', '.join(collect)))

            #chain_growth = float(height[-1]-height[0]) / float(timestamp[-1]-timestamp[0]) * 1000000.0
            chain_growth = float(height[-1]) / 3600.0 
            tot_chain_growth_rates += chain_growth
        print("average chain growth: {}".format(tot_chain_growth_rates / len(d['victims'])))
