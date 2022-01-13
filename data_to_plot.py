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

# Expt parameters
args = parser.parse_args()

def to_dots(x, y):
    if len(x) != len(y):
        print("error")
        os.exit(0)
    series = []
    for i in range(len(x)):
        o = "({}, {})".format(x[i], y[i])
        series.append(o)
    return ' '.join(series)


if __name__ == "__main__":
    with open(args.input) as f:
        d = json.load(f)
        template = """\\addplot [myparula11] coordinates {{
                {}
        }};"""
        first_chain_switch = d['first_chain_switch']
        for k in d['victims']:
            height = d['victims'][k]['chain_growth']['height']
            timestamp = d['victims'][k]['chain_growth']['timestamp']
            l = len(height)
            x = [(t-first_chain_switch)/1000000.0 for t in timestamp]
            print(template.format(to_dots(x, height)))
